// Package ds holds the Datastar adapter, S12.
//
// It provides the server sent event stream, the element patch, the signal
// patch and the script helper. It renders a component into the stream, so a
// page and a patch share one template.
//
// The frames follow the Datastar SDK specification of version 1.0. That
// version carries two events: datastar-patch-elements and
// datastar-patch-signals. It carries no execute script event, so ExecuteScript
// patches a script element into the body, which is the shape that the
// specification states.
//
// The package is an optional import. The view package does not import it, and
// the scaffolded ui package does not import it. A component reads Partial and
// renders a fragment, so one template serves both shapes. See the SDD, S12.
package ds

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/alternayte/avero/router"
)

// The names of the Datastar events. See the Datastar SDK specification,
// version 1.0.
const (
	// EventPatchElements patches elements into the document.
	EventPatchElements = "datastar-patch-elements"
	// EventPatchSignals patches values into the signal store.
	EventPatchSignals = "datastar-patch-signals"
)

// Mode states how a patch places its elements.
type Mode string

// The modes of an element patch. The default is outer, which the frame omits.
const (
	// ModeOuter replaces the element that the selector names.
	ModeOuter Mode = "outer"
	// ModeInner replaces the children of the element.
	ModeInner Mode = "inner"
	// ModeReplace replaces the element and keeps no attribute.
	ModeReplace Mode = "replace"
	// ModePrepend puts the elements before the first child.
	ModePrepend Mode = "prepend"
	// ModeAppend puts the elements after the last child.
	ModeAppend Mode = "append"
	// ModeBefore puts the elements before the element.
	ModeBefore Mode = "before"
	// ModeAfter puts the elements after the element.
	ModeAfter Mode = "after"
	// ModeRemove removes the element that the selector names.
	ModeRemove Mode = "remove"
)

// The names that Datastar sends on a request.
const (
	// RequestHeader marks a request that Datastar made.
	RequestHeader = "Datastar-Request"
	// SignalsParam carries the signals of a GET request.
	SignalsParam = "datastar"
)

// Partial reports whether Datastar made this request. A component renders one
// fragment for a partial request and the whole page for any other.
func Partial(c *router.Ctx) bool {
	return c.Request().Header.Get(RequestHeader) == "true"
}

// ReadSignals fills v from the signals of the request. A GET request carries
// them in the datastar query parameter, and any other method carries them in
// the body as JSON.
//
//	var in struct{ Count int `json:"count"` }
//	if err := ds.ReadSignals(c, &in); err != nil {
//	    return nil, err
//	}
func ReadSignals(c *router.Ctx, v any) error {
	req := c.Request()
	if req.Method == http.MethodGet || req.Method == http.MethodHead {
		raw := req.URL.Query().Get(SignalsParam)
		if raw == "" {
			return nil
		}
		if err := json.Unmarshal([]byte(raw), v); err != nil {
			return fault(fmt.Sprintf("the %s parameter does not parse as JSON: %v", SignalsParam, err),
				"Prove the data-signals attribute of the element that starts the request")
		}
		return nil
	}
	body, err := io.ReadAll(io.LimitReader(req.Body, MaxSignals))
	if err != nil {
		return fault(fmt.Sprintf("the request body does not read: %v", err),
			"Send the signals again")
	}
	if len(strings.TrimSpace(string(body))) == 0 {
		return nil
	}
	if err := json.Unmarshal(body, v); err != nil {
		return fault(fmt.Sprintf("the request body does not parse as JSON: %v", err),
			"Send the signals with a Datastar action, which writes the JSON body")
	}
	return nil
}

// MaxSignals is the largest signal body that ReadSignals reads.
const MaxSignals = 1 << 20
