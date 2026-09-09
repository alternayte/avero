package main

import (
	"io/fs"
	"net/http"

	"github.com/alternayte/avero"
	"github.com/alternayte/drel"

	"orders/internal/features/posts"
	postmigrations "orders/internal/features/posts/migrations"
)

// wire builds the router and the module set.
//
// An inspection command passes a nil engine and a zero configuration, because
// a route registration touches no database and needs no key.
func wire(engine *drel.Engine, cfg Config) (*avero.Router, *avero.ModuleSet, error) {
	r := avero.NewRouter()
	r.Use(avero.RequestID())
	r.Use(avero.Recover(nil))
	if engine != nil {
		r.Use(avero.Transaction(engine))
	}

	// The service answers JSON, so the root path states the name of the
	// service.
	r.Get("/{$}", func(c *avero.Ctx) (avero.Response, error) {
		return avero.JSON(http.StatusOK, map[string]string{"service": "orders"}), nil
	})

	modules := avero.Modules(posts.New(engine))
	if err := modules.Attach(r); err != nil {
		return nil, nil, err
	}
	return r, modules, nil
}

// migrationSets holds the migrations of each feature slice. drel merges them
// in version order, and the binary carries them. See drel.yaml.
func migrationSets() []fs.FS {
	return []fs.FS{postmigrations.FS}
}
