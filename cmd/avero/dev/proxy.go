package dev

import (
	"bytes"
	_ "embed"
	"fmt"
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

// client holds dev/client.js. The proxy adds it to an HTML answer in
// development, so the application carries no development code and a production
// build holds none of it. See S15.
//
//go:embed client.js
var client []byte

// Client returns the reload client. `avero dev` adds it to an HTML answer.
func Client() []byte { return client }

// proxy forwards a request to the application and adds the reload client.
type proxy struct {
	reverse *httputil.ReverseProxy
	inject  bool

	mu   sync.RWMutex
	gate chan struct{}
}

// newProxy builds the proxy in front of the application.
func newProxy(childPort int, inject bool) *proxy {
	target := &url.URL{Scheme: "http", Host: "127.0.0.1:" + strconv.Itoa(childPort)}
	p := &proxy{inject: inject}
	p.reverse = &httputil.ReverseProxy{
		Rewrite: func(r *httputil.ProxyRequest) {
			r.SetURL(target)
			r.Out.Host = r.In.Host
		},
		ModifyResponse: p.modify,
		ErrorHandler: func(w http.ResponseWriter, _ *http.Request, err error) {
			http.Error(w, fmt.Sprintf("avero dev: the application does not answer: %v", err),
				http.StatusBadGateway)
		},
	}
	return p
}

// hold stops the proxy from forwarding. The loop calls it before a restart, so
// a request waits instead of failing and the page keeps its state.
func (p *proxy) hold() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.gate == nil {
		p.gate = make(chan struct{})
	}
}

// release lets the waiting requests through.
func (p *proxy) release() {
	p.mu.Lock()
	gate := p.gate
	p.gate = nil
	p.mu.Unlock()
	if gate != nil {
		close(gate)
	}
}

// wait blocks while the loop restarts the application.
func (p *proxy) wait() {
	p.mu.RLock()
	gate := p.gate
	p.mu.RUnlock()
	if gate == nil {
		return
	}
	select {
	case <-gate:
	case <-time.After(RequestWait):
	}
}

// ServeHTTP forwards one request.
func (p *proxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	p.wait()
	p.reverse.ServeHTTP(w, r)
}

// modify adds the reload client to an HTML answer.
func (p *proxy) modify(res *http.Response) error {
	if !p.inject || !strings.HasPrefix(res.Header.Get("Content-Type"), "text/html") {
		return nil
	}
	body, err := io.ReadAll(res.Body)
	if err != nil {
		return err
	}
	_ = res.Body.Close()

	script := append([]byte("<script>"), client...)
	script = append(script, []byte("</script>")...)
	if i := bytes.LastIndex(body, []byte("</body>")); i >= 0 {
		body = append(body[:i], append(script, body[i:]...)...)
	} else {
		body = append(body, script...)
	}

	res.Body = io.NopCloser(bytes.NewReader(body))
	res.Header.Set("Content-Length", strconv.Itoa(len(body)))
	res.ContentLength = int64(len(body))
	return nil
}
