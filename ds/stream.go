package ds

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"

	"github.com/alternayte/avero/internal/sse"
	"github.com/alternayte/avero/router"
	"github.com/alternayte/avero/view"
)

// ErrClosed states that the client closed the stream. A patch returns it after
// a disconnect, so the handler stops and the goroutine ends.
var ErrClosed = sse.ErrClosed

// Stream writes Datastar frames to one client.
type Stream struct {
	out *sse.Stream
	ctx *router.Ctx
}

// Done returns a channel that closes when the client disconnects.
//
//	select {
//	case <-s.Done():
//	    return nil
//	case row := <-rows:
//	    return s.PatchElements(rows.View(row))
//	}
func (s *Stream) Done() <-chan struct{} { return s.out.Done() }

// Ctx returns the request that opened the stream.
func (s *Stream) Ctx() *router.Ctx { return s.ctx }

// Open returns a Response that opens a server sent event stream and runs fn.
//
// The handler holds the request until fn returns, so fn must select on Done.
// A disconnect makes every patch return ErrClosed, so a loop that ignores Done
// still ends.
//
//	r.Get("/updates", func(c *router.Ctx) (router.Response, error) {
//	    return ds.Open(func(s *ds.Stream) error {
//	        for {
//	            select {
//	            case <-s.Done():
//	                return nil
//	            case row := <-rows:
//	                if err := s.PatchElements(views.Row(row)); err != nil {
//	                    return err
//	                }
//	            }
//	        }
//	    }), nil
//	})
func Open(fn func(s *Stream) error) router.Response { return streamResponse{fn: fn} }

// streamResponse opens the stream when the router writes the response. The
// transaction of the request commits before the first frame, because the
// router writes a response after the commit. See the SDD, S4.
type streamResponse struct {
	fn func(s *Stream) error
}

// Status returns 200. A stream carries its faults in its frames.
func (r streamResponse) Status() int { return 200 }

// Write opens the stream and runs the function of the handler.
func (r streamResponse) Write(c *router.Ctx) error {
	out, err := sse.New(c.Writer(), c.Request())
	if err != nil {
		return err
	}
	if err := r.fn(&Stream{out: out, ctx: c}); err != nil {
		if errors.Is(err, ErrClosed) {
			// The client left. That is an end, not a fault.
			return nil
		}
		return err
	}
	return nil
}

// patch holds the options of one element patch.
type patch struct {
	selector       string
	mode           Mode
	viewTransition bool
}

// Option configures one element patch.
type Option func(*patch)

// WithSelector names the element that the patch changes. An empty selector
// leaves the placement to the identifier of the elements.
func WithSelector(selector string) Option {
	return func(p *patch) { p.selector = selector }
}

// WithMode states how the patch places its elements. The default is outer.
func WithMode(m Mode) Option { return func(p *patch) { p.mode = m } }

// WithViewTransition asks the browser for a view transition.
func WithViewTransition() Option { return func(p *patch) { p.viewTransition = true } }

// PatchElements renders a component and patches it into the document.
//
// The component is a templ component or any other view.Component, so a page
// and a patch share one template.
func (s *Stream) PatchElements(c view.Component, opts ...Option) error {
	var buf bytes.Buffer
	if err := c.Render(s.out.Context(), &buf); err != nil {
		return err
	}
	return s.PatchHTML(buf.String(), opts...)
}

// PatchHTML patches markup into the document. PatchElements renders a
// component and calls it.
func (s *Stream) PatchHTML(html string, opts ...Option) error {
	p := patch{}
	for _, opt := range opts {
		opt(&p)
	}
	return s.out.Send(sse.Event{Name: EventPatchElements, Data: elementLines(p, html)})
}

// RemoveElements removes every element that the selector names.
func (s *Stream) RemoveElements(selector string) error {
	return s.PatchHTML("", WithSelector(selector), WithMode(ModeRemove))
}

// ExecuteScript runs a script in the browser.
//
// Version 1.0 of the Datastar specification carries no execute script event.
// The script becomes a script element that the patch appends to the body, and
// the effect attribute removes the element after it runs.
func (s *Stream) ExecuteScript(script string, opts ...Option) error {
	el := `<script data-effect="el.remove()">` + script + `</script>`
	return s.PatchHTML(el, append([]Option{
		WithSelector("body"), WithMode(ModeAppend),
	}, opts...)...)
}

// signals holds the options of one signal patch.
type signals struct {
	onlyIfMissing bool
}

// SignalOption configures one signal patch.
type SignalOption func(*signals)

// WithOnlyIfMissing keeps a signal that the store already holds.
func WithOnlyIfMissing() SignalOption {
	return func(s *signals) { s.onlyIfMissing = true }
}

// PatchSignals encodes v as JSON and patches it into the signal store.
//
//	return s.PatchSignals(map[string]any{"count": n})
func (s *Stream) PatchSignals(v any, opts ...SignalOption) error {
	cfg := signals{}
	for _, opt := range opts {
		opt(&cfg)
	}
	body, err := json.Marshal(v)
	if err != nil {
		return fault("the signals do not encode as JSON",
			"Pass a value that encoding/json accepts, such as a map or a struct with tags")
	}
	data := make([]string, 0, 2)
	if cfg.onlyIfMissing {
		data = append(data, "onlyIfMissing true")
	}
	data = append(data, "signals "+string(body))
	return s.out.Send(sse.Event{Name: EventPatchSignals, Data: data})
}

// elementLines returns the data lines of an element patch, in the order that
// the specification states: the selector, the mode, the view transition, then
// the elements.
func elementLines(p patch, html string) []string {
	data := make([]string, 0, 4)
	if p.selector != "" {
		data = append(data, "selector "+p.selector)
	}
	if p.mode != "" && p.mode != ModeOuter {
		data = append(data, "mode "+string(p.mode))
	}
	if p.viewTransition {
		data = append(data, "useViewTransition true")
	}
	if strings.TrimSpace(html) != "" {
		// The specification repeats the key on each line, because one data
		// line cannot carry a new line.
		for _, line := range strings.Split(strings.ReplaceAll(html, "\r\n", "\n"), "\n") {
			data = append(data, "elements "+line)
		}
	}
	return data
}
