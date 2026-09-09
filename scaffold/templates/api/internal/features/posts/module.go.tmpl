// Package posts holds the posts feature of the service. It imports no other
// feature.
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

// Routes registers the JSON routes of the feature.
func (m *Module) Routes(r *avero.Router) {
	r.Get("/posts", avero.In(m.List))
	r.Post("/posts", avero.In(m.Create))
	r.Get("/posts/{id}", avero.In(m.Show))
	r.Delete("/posts/{id}", avero.In(m.Delete))
}

// Describe states what the feature contributes. `avero schema` reads it.
func (m *Module) Describe() avero.Description {
	return avero.Description{
		Name: "posts",
		Models: []avero.ModelDesc{{
			Name:  "Post",
			Table: "posts",
			Fields: []avero.FieldDesc{
				{Name: "ID", Type: "uuid.UUID"},
				{Name: "Title", Type: "string"},
				{Name: "Body", Type: "string"},
				{Name: "CreatedAt", Type: "time.Time"},
				{Name: "UpdatedAt", Type: "time.Time"},
			},
		}},
	}
}
