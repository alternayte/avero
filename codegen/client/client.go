// Package client holds the HTTP client generator and the runtime that its
// output calls, S13.
//
// The declaration is an interface with comment directives. `avero generate`
// reads the directives one time and writes a plain implementation. The
// generated file imports no reflect, and no call path reads a struct tag. See
// design rule 2 and AN-2.
//
//	//avero:client base="https://api.example.com" auth="bearer"
//	type Example interface {
//	    //avero:GET /things/{id}
//	    GetThing(ctx context.Context, id string) (Thing, error)
//	}
//
// The runtime carries the timeout, the retry with backoff, the circuit breaker
// and the OTel span of each call. A status outside the two hundreds becomes a
// StatusError, which keeps the response reachable through errors.As.
package client

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// The defaults of one client. An option replaces each one.
const (
	// DefaultTimeout bounds one attempt.
	DefaultTimeout = 10 * time.Second
	// DefaultAttempts is the number of attempts of one call, the first one
	// included.
	DefaultAttempts = 3
	// DefaultBackoff is the delay before the second attempt. Each further
	// delay doubles it.
	DefaultBackoff = 100 * time.Millisecond
	// MaxBackoff bounds one delay.
	MaxBackoff = 2 * time.Second
	// TracerName names the instrumentation of the client.
	TracerName = "github.com/alternayte/avero/codegen/client"
	// MaxErrorBody is the number of bytes of a failed response that a
	// StatusError keeps.
	MaxErrorBody = 1 << 20
)

// Auth states the authentication that a client sends.
type Auth string

// The authentication shapes that a directive names.
const (
	// AuthNone sends no credential.
	AuthNone Auth = "none"
	// AuthBearer sends the token in the Authorization header.
	AuthBearer Auth = "bearer"
	// AuthBasic sends the user and the password in the Authorization
	// header.
	AuthBasic Auth = "basic"
)

// Doer sends one request. The standard http.Client satisfies it, and a test
// passes its own.
type Doer interface {
	Do(req *http.Request) (*http.Response, error)
}

// Client is the runtime of one generated client. The generated code holds it
// as a field and calls Do.
type Client struct {
	name     string
	base     *url.URL
	http     Doer
	header   http.Header
	auth     Auth
	token    string
	user     string
	password string
	timeout  time.Duration
	attempts int
	backoff  time.Duration
	breaker  *Breaker
	tracer   trace.Tracer
	sleep    func(ctx context.Context, d time.Duration) error
	baseErr  error
}

// Option configures a client.
type Option func(*Client)

// WithBaseURL sets the address that every path joins. The generated
// constructor passes the address of the directive, and an option after it
// replaces the address, so a test points the client at httptest.
func WithBaseURL(raw string) Option {
	return func(c *Client) { c.baseRaw(raw) }
}

// WithHTTPClient sets the sender. A test passes its own.
func WithHTTPClient(d Doer) Option { return func(c *Client) { c.http = d } }

// WithHeader adds a header to every request.
func WithHeader(name, value string) Option {
	return func(c *Client) { c.header.Set(name, value) }
}

// WithAuth states the authentication shape. The generated constructor passes
// the shape of the directive.
func WithAuth(a Auth) Option { return func(c *Client) { c.auth = a } }

// WithToken sets the bearer token.
func WithToken(token string) Option { return func(c *Client) { c.token = token } }

// WithBasicAuth sets the user and the password.
func WithBasicAuth(user, password string) Option {
	return func(c *Client) { c.user, c.password = user, password }
}

// WithTimeout bounds one attempt. A zero value keeps the default.
func WithTimeout(d time.Duration) Option {
	return func(c *Client) {
		if d > 0 {
			c.timeout = d
		}
	}
}

// WithAttempts sets the number of attempts of one call, the first one
// included. A value below one keeps the default.
func WithAttempts(n int) Option {
	return func(c *Client) {
		if n >= 1 {
			c.attempts = n
		}
	}
}

// WithBackoff sets the delay before the second attempt. Each further delay
// doubles it, up to MaxBackoff.
func WithBackoff(d time.Duration) Option {
	return func(c *Client) {
		if d > 0 {
			c.backoff = d
		}
	}
}

// WithBreaker sets the circuit breaker. A nil breaker opens no circuit.
func WithBreaker(b *Breaker) Option { return func(c *Client) { c.breaker = b } }

// WithTracer sets the tracer. The default reads the global provider.
func WithTracer(t trace.Tracer) Option { return func(c *Client) { c.tracer = t } }

// baseRaw parses and stores the base address. A fault stays until New reads
// it, so New reports it with the repair.
func (c *Client) baseRaw(raw string) {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme == "" || u.Host == "" {
		c.base = nil
		c.baseErr = fmt.Errorf("the base address %q is not an absolute URL", raw)
		return
	}
	c.base = u
	c.baseErr = nil
}

// New builds the runtime of one client. The generated constructor calls it.
func New(name string, opts ...Option) (*Client, error) {
	c := &Client{
		name:     name,
		http:     &http.Client{},
		header:   http.Header{},
		auth:     AuthNone,
		timeout:  DefaultTimeout,
		attempts: DefaultAttempts,
		backoff:  DefaultBackoff,
		breaker:  NewBreaker(BreakerConfig{}),
		sleep:    sleep,
	}
	for _, opt := range opts {
		opt(c)
	}
	if c.baseErr != nil {
		return nil, fault(c.baseErr.Error(),
			"Pass an address with a scheme and a host, such as https://api.example.com")
	}
	if c.base == nil {
		return nil, fault(fmt.Sprintf("the client %q has no base address", name),
			"Write base=\"https://api.example.com\" in the //avero:client directive, or pass WithBaseURL")
	}
	if c.tracer == nil {
		c.tracer = otel.Tracer(TracerName)
	}
	return c, nil
}

// Request states one call. The generated code fills it.
type Request struct {
	// Op names the method of the interface, such as Example.GetThing. The
	// span carries it.
	Op string
	// Method is the HTTP method.
	Method string
	// Path is the path of the call, with every parameter already in place.
	Path string
	// Query holds the query parameters.
	Query url.Values
	// Header holds the headers of this call.
	Header http.Header
	// Body becomes the JSON body. A nil value sends no body.
	Body any
	// Idempotent states that a retry is safe. The generated code sets it
	// from the HTTP method, and a directive can state it for a POST.
	Idempotent bool
}

// Do sends the request, retries it under the policy of the client, and decodes
// the body into out. A nil out reads no body.
func (c *Client) Do(ctx context.Context, req Request, out any) error {
	body, err := encode(req.Body)
	if err != nil {
		return err
	}
	target, err := c.resolve(req)
	if err != nil {
		return err
	}

	ctx, span := c.tracer.Start(ctx, c.spanName(req), trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			attribute.String("http.request.method", req.Method),
			attribute.String("url.full", target.String()),
			attribute.String("server.address", target.Hostname()),
			attribute.String("rpc.method", req.Op),
		))
	defer span.End()

	err = c.call(ctx, req, target, body, out, span)
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
	}
	return err
}

// call runs the attempts of one call.
func (c *Client) call(ctx context.Context, req Request, target *url.URL, body []byte, out any, span trace.Span) error {
	var last error
	for attempt := 1; attempt <= c.attempts; attempt++ {
		if err := c.breaker.Allow(); err != nil {
			return err
		}
		res, err := c.send(ctx, req, target, body)
		switch {
		case err != nil:
			c.breaker.Failure()
			last = err
		default:
			status := res.StatusCode
			if status >= 200 && status < 300 {
				c.breaker.Success()
				span.SetAttributes(attribute.Int("http.response.status_code", status))
				return decode(res, out)
			}
			statusErr := newStatusError(req, target, res)
			if status >= 500 || status == http.StatusTooManyRequests {
				c.breaker.Failure()
			} else {
				// A fault of the request is an answer, not an outage.
				c.breaker.Success()
			}
			span.SetAttributes(attribute.Int("http.response.status_code", status))
			last = statusErr
		}
		if attempt == c.attempts || !c.retryable(req, last) {
			return last
		}
		if err := c.sleep(ctx, c.delay(attempt, last)); err != nil {
			return errors.Join(last, err)
		}
	}
	return last
}

// send performs one attempt.
func (c *Client) send(ctx context.Context, req Request, target *url.URL, body []byte) (*http.Response, error) {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}
	r, err := http.NewRequestWithContext(ctx, req.Method, target.String(), reader)
	if err != nil {
		return nil, fault(fmt.Sprintf("the request of %s does not build: %v", req.Op, err),
			"Prove the base address and the path of the directive")
	}
	for name, values := range c.header {
		for _, v := range values {
			r.Header.Add(name, v)
		}
	}
	for name, values := range req.Header {
		for _, v := range values {
			r.Header.Add(name, v)
		}
	}
	if body != nil && r.Header.Get("Content-Type") == "" {
		r.Header.Set("Content-Type", "application/json")
	}
	if r.Header.Get("Accept") == "" {
		r.Header.Set("Accept", "application/json")
	}
	c.authorize(r)

	res, err := c.http.Do(r)
	if err != nil {
		return nil, &TransportError{Op: req.Op, URL: target.String(), Err: err}
	}
	return res, nil
}

// authorize writes the credential of the client.
func (c *Client) authorize(r *http.Request) {
	switch c.auth {
	case AuthBearer:
		if c.token != "" {
			r.Header.Set("Authorization", "Bearer "+c.token)
		}
	case AuthBasic:
		if c.user != "" {
			r.SetBasicAuth(c.user, c.password)
		}
	case AuthNone:
	}
}

// resolve joins the base address, the path and the query.
//
// The path of a Request is already escaped, because the generated code escapes
// each parameter. The URL therefore carries the decoded form and the escaped
// form, so no byte is escaped two times.
func (c *Client) resolve(req Request) (*url.URL, error) {
	ref, err := url.Parse(req.Path)
	if err != nil {
		return nil, fault(fmt.Sprintf("the path %q of %s does not parse", req.Path, req.Op),
			"Prove the path of the directive, and escape a parameter with client.Escape")
	}
	target := *c.base
	target.Path = strings.TrimSuffix(c.base.Path, "/") + ref.Path
	target.RawPath = strings.TrimSuffix(c.base.EscapedPath(), "/") + ref.EscapedPath()
	if target.RawPath == target.Path {
		target.RawPath = ""
	}
	if len(req.Query) > 0 {
		target.RawQuery = req.Query.Encode()
	}
	return &target, nil
}

// spanName returns the name of the span. The convention of OTel is the method
// and the route.
func (c *Client) spanName(req Request) string { return req.Method + " " + c.name + "." + req.Op }

// retryable reports whether another attempt can help.
func (c *Client) retryable(req Request, err error) bool {
	if !req.Idempotent {
		return false
	}
	var transport *TransportError
	if errors.As(err, &transport) {
		return true
	}
	var status *StatusError
	if errors.As(err, &status) {
		return status.Status >= 500 || status.Status == http.StatusTooManyRequests
	}
	return false
}

// delay returns the wait before the next attempt. It doubles the backoff and
// it reads Retry-After when the server sent one.
func (c *Client) delay(attempt int, err error) time.Duration {
	var status *StatusError
	if errors.As(err, &status) {
		if after := status.RetryAfter(); after > 0 {
			return after
		}
	}
	d := c.backoff << (attempt - 1)
	if d > MaxBackoff {
		d = MaxBackoff
	}
	return d
}

// sleep waits for d, or returns when the caller cancels.
func sleep(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return nil
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

// encode turns the body into JSON.
func encode(body any) ([]byte, error) {
	if body == nil {
		return nil, nil
	}
	b, err := json.Marshal(body)
	if err != nil {
		return nil, fault("the request body does not encode as JSON",
			"Give the body type JSON tags that encoding/json accepts")
	}
	return b, nil
}

// decode reads the response into out.
func decode(res *http.Response, out any) error {
	defer func() { _, _ = io.Copy(io.Discard, res.Body); _ = res.Body.Close() }()
	if out == nil || res.StatusCode == http.StatusNoContent {
		return nil
	}
	if err := json.NewDecoder(res.Body).Decode(out); err != nil {
		return fault(fmt.Sprintf("the response body does not parse as JSON: %v", err),
			"Prove the shape of the answer. Repair the return type of the method.")
	}
	return nil
}

// NewQuery returns an empty query. The generated code fills it, so the
// generated file imports no net/url.
func NewQuery() url.Values { return url.Values{} }

// NewHeader returns an empty header set. The generated code fills it, so the
// generated file imports no net/http.
func NewHeader() http.Header { return http.Header{} }

// Escape encodes one path parameter.
func Escape(v string) string { return url.PathEscape(v) }

// Int returns the decimal form of a signed value.
func Int(v int64) string { return strconv.FormatInt(v, 10) }

// Uint returns the decimal form of an unsigned value.
func Uint(v uint64) string { return strconv.FormatUint(v, 10) }

// Float returns the shortest form of a floating point value.
func Float(v float64) string { return strconv.FormatFloat(v, 'g', -1, 64) }

// Bool returns true or false.
func Bool(v bool) string { return strconv.FormatBool(v) }

// MustDuration parses a duration and panics on a fault. The generated code
// calls it with a value that the generator already proved, so it cannot panic
// at run time.
func MustDuration(v string) time.Duration {
	d, err := time.ParseDuration(v)
	if err != nil {
		panic("client: the generated duration " + v + " does not parse")
	}
	return d
}
