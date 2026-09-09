package avero

import "github.com/alternayte/avero/router"

// The typed registration of a route.
//
// A handler returns the thing that it answers, as an ordinary Go function
// does. The type of the input and the type of the answer come from its
// signature, so the compiler holds them, and the description of the API reads
// them. A route needs no wrapper and no comment.
//
//	func (m *Module) Show(c *avero.Ctx, in ShowInput) (View, error)
//
//	func (m *Module) Routes(r *avero.Router) {
//	    avero.Get(r, "/posts", m.List)
//	    avero.Post(r, "/posts", m.Create)
//	    avero.Get(r, "/posts/{id}", m.Show)
//	    avero.Delete(r, "/posts/{id}", m.Delete)
//	}

// Get registers a typed GET route.
func Get[T any, P router.Input[T], V any](r *Router, pattern string,
	fn func(c *Ctx, in T) (V, error), opts ...OpOption,
) {
	router.Get[T, P](r, pattern, fn, opts...)
}

// Post registers a typed POST route. The success answer is 201.
func Post[T any, P router.Input[T], V any](r *Router, pattern string,
	fn func(c *Ctx, in T) (V, error), opts ...OpOption,
) {
	router.Post[T, P](r, pattern, fn, opts...)
}

// Put registers a typed PUT route.
func Put[T any, P router.Input[T], V any](r *Router, pattern string,
	fn func(c *Ctx, in T) (V, error), opts ...OpOption,
) {
	router.Put[T, P](r, pattern, fn, opts...)
}

// Patch registers a typed PATCH route.
func Patch[T any, P router.Input[T], V any](r *Router, pattern string,
	fn func(c *Ctx, in T) (V, error), opts ...OpOption,
) {
	router.Patch[T, P](r, pattern, fn, opts...)
}

// Delete registers a typed DELETE route.
func Delete[T any, P router.Input[T], V any](r *Router, pattern string,
	fn func(c *Ctx, in T) (V, error), opts ...OpOption,
) {
	router.Delete[T, P](r, pattern, fn, opts...)
}
