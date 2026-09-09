package tasks

import (
	"net/http"

	"github.com/alternayte/avero"
)

// View is one task as the API returns it.
type View struct {
	// ID identifies the task.
	ID string `json:"id"`
	// Title is the text that a person reads.
	Title string `json:"title"`
	// Done states whether the task is complete.
	Done bool `json:"done"`
}

// List answers every task.
func (m *Module) List(c *avero.Ctx, in ListInput) (avero.Response, error) {
	tasks, err := m.store.List(c.Context(), in.Search)
	if err != nil {
		return nil, err
	}
	out := make([]View, 0, len(tasks))
	for _, t := range tasks {
		out = append(out, View{ID: t.ID, Title: t.Title, Done: t.Done})
	}
	return avero.JSON(http.StatusOK, map[string]any{"tasks": out}), nil
}

// Create writes one task.
func (m *Module) Create(c *avero.Ctx, in CreateInput) (avero.Response, error) {
	id, err := m.store.Create(c.Context(), in.Title)
	if err != nil {
		return nil, err
	}
	return avero.JSON(http.StatusCreated, View{ID: id, Title: in.Title}), nil
}

// Update marks one task complete, or open again.
func (m *Module) Update(c *avero.Ctx, in UpdateInput) (avero.Response, error) {
	task, found, err := m.store.SetDone(c.Context(), in.ID, in.Done)
	if err != nil {
		return nil, err
	}
	if !found {
		return avero.JSON(http.StatusNotFound, map[string]string{"error": "the task is absent"}), nil
	}
	return avero.JSON(http.StatusOK, View{ID: task.ID, Title: task.Title, Done: task.Done}), nil
}

// Delete removes one task.
func (m *Module) Delete(c *avero.Ctx, in DeleteInput) (avero.Response, error) {
	if err := m.store.Delete(c.Context(), in.ID); err != nil {
		return nil, err
	}
	return avero.NoContent(), nil
}
