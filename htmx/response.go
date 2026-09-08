package htmx

import (
	"bytes"
	"encoding/json"
	"net/http"

	"github.com/alternayte/avero/router"
	"github.com/alternayte/avero/view"
)

// ContentType is the content type of a fragment.
const ContentType = "text/html; charset=utf-8"

// The headers that htmx reads on a response.
const (
	// LocationHeader asks htmx for a client side navigation.
	LocationHeader = "HX-Location"
	// PushURLHeader pushes an address into the history.
	PushURLHeader = "HX-Push-Url"
	// ReplaceURLHeader replaces the address of the history.
	ReplaceURLHeader = "HX-Replace-Url"
	// RedirectHeader asks the browser for a whole page navigation.
	RedirectHeader = "HX-Redirect"
	// RefreshHeader asks the browser for a whole page reload.
	RefreshHeader = "HX-Refresh"
	// ReswapHeader states how htmx swaps the answer.
	ReswapHeader = "HX-Reswap"
	// RetargetHeader states the element that htmx swaps.
	RetargetHeader = "HX-Retarget"
	// ReselectHeader states the part of the answer that htmx takes.
	ReselectHeader = "HX-Reselect"
	// TriggerAfterSwapHeader triggers an event after the swap.
	TriggerAfterSwapHeader = "HX-Trigger-After-Swap"
	// TriggerAfterSettleHeader triggers an event after the settle.
	TriggerAfterSettleHeader = "HX-Trigger-After-Settle"
)

// fragment renders components into one answer.
type fragment struct {
	code       int
	components []view.Component
}

// Status returns the code that Write sends.
func (f fragment) Status() int { return f.code }

// Write renders every component into a buffer and then sends it. The buffer
// keeps a render fault away from the client.
func (f fragment) Write(c *router.Ctx) error {
	var buf bytes.Buffer
	ctx := view.RenderContext(c)
	for _, component := range f.components {
		if err := component.Render(ctx, &buf); err != nil {
			return err
		}
	}
	c.Header().Set("Content-Type", ContentType)
	c.WriteHeader(f.code)
	_, err := c.Writer().Write(buf.Bytes())
	return err
}

// Fragment renders one component or several with 200.
//
// An out of band swap is an attribute of the template, so a second component
// carries hx-swap-oob and reaches another place on the page.
//
//	return htmx.Fragment(views.Row(row), views.Count(n)), nil
func Fragment(components ...view.Component) router.Response {
	return fragment{code: http.StatusOK, components: components}
}

// FragmentStatus renders components with this status, for example 422.
func FragmentStatus(code int, components ...view.Component) router.Response {
	return fragment{code: code, components: components}
}

// Retarget states the element that htmx swaps.
func Retarget(c *router.Ctx, selector string) { c.Header().Set(RetargetHeader, selector) }

// Reswap states how htmx swaps the answer, such as beforeend.
func Reswap(c *router.Ctx, spec string) { c.Header().Set(ReswapHeader, spec) }

// Reselect states the part of the answer that htmx takes.
func Reselect(c *router.Ctx, selector string) { c.Header().Set(ReselectHeader, selector) }

// PushURL pushes an address into the history of the browser.
func PushURL(c *router.Ctx, url string) { c.Header().Set(PushURLHeader, url) }

// ReplaceURL replaces the address of the history.
func ReplaceURL(c *router.Ctx, url string) { c.Header().Set(ReplaceURLHeader, url) }

// Refresh asks the browser to load the whole page again.
func Refresh(c *router.Ctx) { c.Header().Set(RefreshHeader, "true") }

// Trigger asks htmx to raise an event after the answer arrives.
//
// A string names one event. A map names one event for each key and carries a
// detail, which encodes as JSON.
func Trigger(c *router.Ctx, event any) error {
	return setEvent(c, TriggerHeader, event)
}

// TriggerAfterSwap raises an event after the swap.
func TriggerAfterSwap(c *router.Ctx, event any) error {
	return setEvent(c, TriggerAfterSwapHeader, event)
}

// TriggerAfterSettle raises an event after the settle.
func TriggerAfterSettle(c *router.Ctx, event any) error {
	return setEvent(c, TriggerAfterSettleHeader, event)
}

// setEvent writes one trigger header.
func setEvent(c *router.Ctx, header string, event any) error {
	if name, ok := event.(string); ok {
		c.Header().Set(header, name)
		return nil
	}
	body, err := json.Marshal(event)
	if err != nil {
		return fault("the event detail does not encode as JSON",
			"Pass a string, or a map that encoding/json accepts")
	}
	c.Header().Set(header, string(body))
	return nil
}

// Redirect asks the browser for a whole page navigation. htmx reads the header
// of a 200 answer, so the response carries no status of its own.
func Redirect(c *router.Ctx, url string) router.Response {
	c.Header().Set(RedirectHeader, url)
	return router.Empty(http.StatusOK)
}

// Location asks htmx for a client side navigation, which loads the page into
// the current document.
func Location(c *router.Ctx, url string) router.Response {
	c.Header().Set(LocationHeader, url)
	return router.Empty(http.StatusOK)
}
