// Package tasks holds the tasks feature of the JSON API that the front end
// calls. It imports no other feature.
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
// the handler, so the description of the API needs no comment.
func (m *Module) Routes(r *avero.Router) {
	avero.Get(r, "/api/tasks", m.List, avero.Summary("List every task"))
	avero.Post(r, "/api/tasks", m.Create, avero.Summary("Write one task"))
	avero.Patch(r, "/api/tasks/{id}", m.Update,
		avero.Summary("Mark one task complete, or open again"),
		avero.Answers[avero.Problem](http.StatusNotFound, "the task does not exist"))
	avero.Delete(r, "/api/tasks/{id}", m.Delete, avero.Summary("Delete one task"))
}

// Describe states what the feature contributes. `avero schema` reads it.
func (m *Module) Describe() avero.Description {
	return avero.Description{
		Name: "tasks",
		Models: []avero.ModelDesc{{
			Name:  "Task",
			Table: "tasks",
			Fields: []avero.FieldDesc{
				{Name: "ID", Type: "uuid.UUID"},
				{Name: "Title", Type: "string"},
				{Name: "Done", Type: "bool"},
				{Name: "CreatedAt", Type: "time.Time"},
				{Name: "UpdatedAt", Type: "time.Time"},
			},
		}},
	}
}
