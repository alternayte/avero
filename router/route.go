package router

import "net/http"

// Route is one registered route. `avero routes` prints it. See AN-3.
type Route struct {
	// Method is the HTTP method, or * for a mounted handler.
	Method string `json:"method"`
	// Pattern is the full pattern, with every group prefix joined.
	Pattern string `json:"pattern"`
	// Handler is the name of the function that serves the route.
	Handler string `json:"handler"`
	// Middleware names the chain, from the outermost to the innermost.
	Middleware []string `json:"middleware"`
	// File is the file that registered the route.
	File string `json:"file"`
	// Line is the line that registered the route.
	Line int `json:"line"`
	// Mounted reports an http.Handler that Mount serves. No transaction
	// surrounds a mounted handler.
	Mounted bool `json:"mounted"`

	handler  Handler
	mws      []Middleware
	mount    http.Handler
	stripped string
}
