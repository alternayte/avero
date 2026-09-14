// Package auth holds the auth-all glue of Avero: the user and the session of
// a request.
//
// The router imports no authentication library. An application that keeps its
// own identity therefore imports the router and never compiles auth-all. An
// application that uses auth-all adapts its middleware with router.Adapt and
// reads the user with this package.
//
// auth-all owns authentication. Avero owns lifecycle, not semantics. See the
// SDD, S4.
package auth

import (
	"context"

	authall "github.com/alternayte/auth-all"
	"github.com/alternayte/auth-all/store"
)

// User returns the signed-in user. It returns nil for an anonymous request.
// auth-all attaches the user with RequireAuth or LoadSession.
//
//	user := auth.User(c)
func User(ctx context.Context) *store.User { return authall.UserFrom(ctx) }

// Session returns the auth-all session. It returns nil for an anonymous
// request.
func Session(ctx context.Context) *store.Session { return authall.SessionFrom(ctx) }
