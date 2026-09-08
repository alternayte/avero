package router

import (
	"context"

	authall "github.com/alternayte/auth-all"
	"github.com/alternayte/auth-all/store"
)

// The router reads the user and the session that auth-all attached. auth-all
// owns authentication. Avero owns lifecycle, not semantics. These two wrappers
// keep the auth-all import in one file.

func authUserFrom(ctx context.Context) *store.User { return authall.UserFrom(ctx) }

func authSessionFrom(ctx context.Context) *store.Session { return authall.SessionFrom(ctx) }
