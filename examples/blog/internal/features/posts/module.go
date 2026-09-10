// Package posts holds the posts feature: its routes, its handlers, its input
// types, its store and its tests. It imports no other feature.
//
// The description of this module needs no hand-written method. `avero
// generate` writes the models from the model package, and the module system
// reads the routes from the Routes method. See AN-3.
package posts

import (
	"io/fs"

	"github.com/alternayte/avero"
	"github.com/alternayte/drel"

	"blog/internal/features/posts/migrations"
)

//go:generate go run github.com/alternayte/avero/internal/cmd/averogen .

// Module is the posts feature.
type Module struct {
	// store reads and writes the posts.
	store *Store
}

// New builds the feature. main.go supplies the engine. See design rule 3.
func New(engine *drel.Engine) *Module {
	return &Module{store: NewStore(engine)}
}

// Name identifies the module in the contribution table.
func (m *Module) Name() string { return "posts" }

// Routes registers the routes of the feature.
func (m *Module) Routes(r *avero.Router) {
	// The root page carries {$}, so it matches the root only. A bare
	// slash would take every path that no other route holds, and it would
	// conflict with the mounted asset handler.
	r.Get("/{$}", avero.In(m.List))
	r.Get("/posts/new", avero.In(m.New))
	r.Post("/posts", avero.In(m.Create))
	r.Get("/posts/{id}", avero.In(m.Show))
	r.Delete("/posts/{id}", avero.In(m.Delete))
}

// Migrations returns the migration files of this feature. The module set
// merges the sets of every module, and drel applies them in version order.
func (m *Module) Migrations() fs.FS { return migrations.FS }
