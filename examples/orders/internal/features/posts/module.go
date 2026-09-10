// Package posts holds the posts feature of the service. It imports no other
// feature.
//
// The description of this module needs no hand-written method. `avero
// generate` writes the models from the model package, and the module system
// reads the routes from the Routes method. See AN-3.
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
// the handler. The comment of each handler states its summary, which
// `avero generate` writes into the generated file.
func (m *Module) Routes(r *avero.Router) {
	avero.Get(r, "/posts", m.List)
	avero.Post(r, "/posts", m.Create)
	// An answer that the signature cannot carry stands in an option, so the
	// description names every case that a client meets.
	avero.Get(r, "/posts/{id}", m.Show,
		avero.Answers[avero.Problem](http.StatusNotFound, "the post does not exist"))
	avero.Delete(r, "/posts/{id}", m.Delete)
}
