// Package posts holds the posts feature of the service. It imports no other
// feature.
package posts

import (
	"net/http"

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
//
// The registration reads the type of the input and the type of the answer from
// the handler, so the description of the API needs no comment.
func (m *Module) Routes(r *avero.Router) {
	avero.Get(r, "/posts", m.List, avero.Summary("List every post"))
	avero.Post(r, "/posts", m.Create, avero.Summary("Write one post"))
	// An answer that the signature cannot carry stands in an option, so the
	// description names every case that a client meets.
	avero.Get(r, "/posts/{id}", m.Show,
		avero.Summary("Answer one post"),
		avero.Answers[avero.Problem](http.StatusNotFound, "the post does not exist"))
	avero.Delete(r, "/posts/{id}", m.Delete, avero.Summary("Delete one post"))
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
