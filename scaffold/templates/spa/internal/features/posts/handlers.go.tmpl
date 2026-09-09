package posts

import (
	"net/http"

	"github.com/alternayte/avero"
)

// View is one post as the API returns it.
type View struct {
	// ID identifies the post.
	ID string `json:"id"`
	// Title is the name that a person reads.
	Title string `json:"title"`
	// Body is the text of the post.
	Body string `json:"body"`
}

// List answers every post.
func (m *Module) List(c *avero.Ctx, in ListInput) (avero.Response, error) {
	posts, err := m.store.List(c.Context(), in.Search)
	if err != nil {
		return nil, err
	}
	out := make([]View, 0, len(posts))
	for _, p := range posts {
		out = append(out, View{ID: p.ID, Title: p.Title, Body: p.Body})
	}
	return avero.JSON(http.StatusOK, map[string]any{"posts": out}), nil
}

// Create writes one post.
func (m *Module) Create(c *avero.Ctx, in CreateInput) (avero.Response, error) {
	id, err := m.store.Create(c.Context(), in.Title, in.Body)
	if err != nil {
		return nil, err
	}
	return avero.JSON(http.StatusCreated, View{ID: id, Title: in.Title, Body: in.Body}), nil
}

// Show answers one post.
func (m *Module) Show(c *avero.Ctx, in ShowInput) (avero.Response, error) {
	post, found, err := m.store.Get(c.Context(), in.ID)
	if err != nil {
		return nil, err
	}
	if !found {
		return avero.JSON(http.StatusNotFound, map[string]string{"error": "the post is absent"}), nil
	}
	return avero.JSON(http.StatusOK, View{ID: post.ID, Title: post.Title, Body: post.Body}), nil
}

// Delete removes one post.
func (m *Module) Delete(c *avero.Ctx, in DeleteInput) (avero.Response, error) {
	if err := m.store.Delete(c.Context(), in.ID); err != nil {
		return nil, err
	}
	return avero.NoContent(), nil
}
