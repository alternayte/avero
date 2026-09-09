// Package posts holds the posts feature of the JSON API that the front end
// calls. It imports no other feature.
package posts

import (
	"github.com/alternayte/avero"
	"github.com/alternayte/drel"
)

//go:generate go run github.com/alternayte/avero/internal/cmd/averogen .

// Module is the posts feature.
type Module struct {
	// store reads and writes the posts.
	store *Store
}

// New builds the feature. main.go supplies the engine. See design rule 3.
func New(engine *drel.Engine) *Module { return &Module{store: NewStore(engine)} }

// Name identifies the module in the contribution table.
func (m *Module) Name() string { return "posts" }

// Routes registers the JSON routes that the front end calls.
func (m *Module) Routes(r *avero.Router) {
	r.Get("/api/posts", avero.In(m.List))
	r.Post("/api/posts", avero.In(m.Create))
	r.Get("/api/posts/{id}", avero.In(m.Show))
	r.Delete("/api/posts/{id}", avero.In(m.Delete))
}

// Describe states what the feature contributes. `avero schema` reads it.
func (m *Module) Describe() avero.Description {
	return avero.Description{
		Name: "posts",
		Models: []avero.ModelDesc{{
			Name:  "Post",
			Table: "posts",
			Fields: []avero.FieldDesc{
				{Name: "ID", Type: "string"},
				{Name: "Title", Type: "string"},
				{Name: "Body", Type: "string"},
				{Name: "CreatedAt", Type: "time.Time"},
			},
		}},
	}
}
