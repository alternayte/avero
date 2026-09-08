package router

import "net/http"

// Middleware wraps a handler. It carries a name, because `avero routes` prints
// the chain of each route. See the SDD, S4.
type Middleware struct {
	// Name identifies the middleware in the routes output.
	Name string
	// Wrap returns a handler that calls next.
	Wrap func(next Handler) Handler
}

// chain applies the middleware in registration order. The first registered
// middleware is the outermost one.
func chain(h Handler, mws []Middleware) Handler {
	for i := len(mws) - 1; i >= 0; i-- {
		h = mws[i].Wrap(h)
	}
	return h
}

// Adapt turns a net/http middleware into an Avero middleware, so that a
// library such as auth-all composes with a typed handler.
//
// The adapted middleware sees a handler that runs the rest of the chain and
// writes nothing. When it calls that handler, Adapt returns the Response that
// the chain produced. When it answers the request itself, for example a 401
// from RequireAuth, Adapt returns a response that writes nothing more, because
// the middleware already wrote the whole answer.
func Adapt(name string, mw func(http.Handler) http.Handler) Middleware {
	return Middleware{
		Name: name,
		Wrap: func(next Handler) Handler {
			return func(c *Ctx) (Response, error) {
				var (
					res    Response
					err    error
					called bool
				)
				inner := http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
					called = true
					c.setRequest(r)
					res, err = next(c)
				})
				mw(inner).ServeHTTP(c.Writer(), c.Request())
				if !called {
					// The middleware answered the request itself.
					return alreadyWritten{}, nil
				}
				return res, err
			}
		},
	}
}

// alreadyWritten reports that a net/http middleware wrote the whole response.
// The router writes nothing more. Its status is 200, so the transaction
// middleware commits: the middleware decided the answer, and a rejection such
// as a 401 leaves nothing to roll back.
type alreadyWritten struct{}

func (alreadyWritten) Status() int      { return http.StatusOK }
func (alreadyWritten) Write(*Ctx) error { return nil }
