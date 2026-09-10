// Package tasks holds the tasks feature of the JSON API that the front end
// calls. It imports no other feature.
//
// The description of this module needs no hand-written method. `avero
// generate` writes the models from the model package, and the module system
// reads the routes from the Routes method. See AN-3.
package tasks

import (
	"net/http"

	"github.com/alternayte/avero"
	"github.com/alternayte/drel"
)

//go:generate go run github.com/alternayte/avero/internal/cmd/averogen .

// Module is the tasks feature.
type Module struct {
	// store reads and writes the tasks.
	store *Store
}

// New builds the feature. main.go supplies the engine. See design rule 3.
func New(engine *drel.Engine) *Module { return &Module{store: NewStore(engine)} }

// Name identifies the module in the contribution table.
func (m *Module) Name() string { return "tasks" }

// Routes registers the JSON routes that the front end calls.
//
// The registration reads the type of the input and the type of the answer from
// the handler. The comment of each handler states its summary, which
// `avero generate` writes into the generated file.
func (m *Module) Routes(r *avero.Router) {
	avero.Get(r, "/api/tasks", m.List)
	avero.Post(r, "/api/tasks", m.Create)
	avero.Patch(r, "/api/tasks/{id}", m.Update,
		avero.Answers[avero.Problem](http.StatusNotFound, "the task does not exist"))
	avero.Delete(r, "/api/tasks/{id}", m.Delete)
}
