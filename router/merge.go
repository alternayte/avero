package router

import "strings"

// Routes returns every route that this router holds, in registration order. It
// reports no fault. Call Report for the sorted table and the faults.
//
// The module system reads it to name the routes of one module. See the SDD,
// S6.
func (r *Router) Routes() []Route {
	out := make([]Route, len(r.reg.routes))
	copy(out, r.reg.routes)
	return out
}

// Merge copies every route and every fault of src into this scope.
//
// The prefix of this scope joins the pattern of each route. The middleware of
// this scope wraps the middleware that src holds. A duplicate pattern becomes
// a fault, in the same form as a direct registration.
//
// The module system builds one router for each module and merges it. A module
// therefore registers its routes one time. See the SDD, S6.
func (r *Router) Merge(src *Router) {
	if src == nil {
		return
	}
	for _, route := range src.Routes() {
		route.Pattern = joinPattern(r.prefix, route.Pattern)
		route.mws = append(append([]Middleware(nil), r.mws...), route.mws...)
		route.Middleware = names(route.mws)
		if route.Mounted {
			route.Pattern = strings.TrimSuffix(route.Pattern, "/") + "/"
			route.stripped = strings.TrimSuffix(route.Pattern, "/")
		}
		r.add(route)
	}
	r.reg.faults = append(r.reg.faults, src.reg.faults...)
}

// Faults returns every registration fault that this router holds, in the order
// that the router found them. The module system reads it to name the module
// that holds a fault. See the SDD, S6.
func (r *Router) Faults() []*Fault {
	out := make([]*Fault, len(r.reg.faults))
	copy(out, r.reg.faults)
	return out
}
