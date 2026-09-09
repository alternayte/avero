// Package tasks holds the tasks feature of the JSON API that the front end
// calls. It imports no other feature.
package tasks

import (
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
func (m *Module) Routes(r *avero.Router) {
	r.Get("/api/tasks", avero.In(m.List))
	r.Post("/api/tasks", avero.In(m.Create))
	r.Patch("/api/tasks/{id}", avero.In(m.Update))
	r.Delete("/api/tasks/{id}", avero.In(m.Delete))
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
