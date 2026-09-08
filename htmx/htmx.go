// Package htmx holds the htmx adapter, S12.
//
// It provides the fragment response, the request helpers and the response
// headers that htmx reads. It renders a component, so a page and a fragment
// share one template.
//
// The package is an optional import. The view package does not import it, and
// the scaffolded ui package does not import it. A component reads Partial and
// renders a fragment, so one template serves both shapes. See the SDD, S12.
//
// The headers follow htmx 2.
package htmx

import "github.com/alternayte/avero/router"

// The headers that htmx sends on a request.
const (
	// RequestHeader marks a request that htmx made.
	RequestHeader = "HX-Request"
	// BoostedHeader marks a request from a boosted link or form.
	BoostedHeader = "HX-Boosted"
	// TargetHeader carries the identifier of the target element.
	TargetHeader = "HX-Target"
	// TriggerHeader carries the identifier of the element that triggered.
	TriggerHeader = "HX-Trigger"
	// TriggerNameHeader carries the name of that element.
	TriggerNameHeader = "HX-Trigger-Name"
	// CurrentURLHeader carries the address of the browser.
	CurrentURLHeader = "HX-Current-URL"
	// PromptHeader carries the answer to hx-prompt.
	PromptHeader = "HX-Prompt"
	// HistoryRestoreHeader marks a history restoration request.
	HistoryRestoreHeader = "HX-History-Restore-Request"
)

// Partial reports whether htmx made this request. A component renders one
// fragment for a partial request and the whole page for any other.
//
//	if htmx.Partial(c) {
//	    return htmx.Fragment(views.Row(row)), nil
//	}
//	return view.View(pages.Rows(rows)), nil
func Partial(c *router.Ctx) bool {
	return c.Request().Header.Get(RequestHeader) == "true"
}

// Boosted reports whether a boosted link or form made this request.
func Boosted(c *router.Ctx) bool {
	return c.Request().Header.Get(BoostedHeader) == "true"
}

// HistoryRestore reports whether htmx restores a page from its history.
func HistoryRestore(c *router.Ctx) bool {
	return c.Request().Header.Get(HistoryRestoreHeader) == "true"
}

// Target returns the identifier of the target element.
func Target(c *router.Ctx) string { return c.Request().Header.Get(TargetHeader) }

// TriggerID returns the identifier of the element that triggered the request.
func TriggerID(c *router.Ctx) string { return c.Request().Header.Get(TriggerHeader) }

// TriggerName returns the name of the element that triggered the request.
func TriggerName(c *router.Ctx) string { return c.Request().Header.Get(TriggerNameHeader) }

// CurrentURL returns the address of the browser.
func CurrentURL(c *router.Ctx) string { return c.Request().Header.Get(CurrentURLHeader) }

// Prompt returns the answer that hx-prompt collected.
func Prompt(c *router.Ctx) string { return c.Request().Header.Get(PromptHeader) }
