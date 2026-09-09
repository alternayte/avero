package posts

import (
	"net/http"

	"github.com/alternayte/avero"

	"orders/internal/features/posts/model"
)

// View is one post as the API returns it. The description of the API names
// this type, and a client reads the same shape. See `avero routes --openapi`.
type View struct {
	// ID identifies the post.
	ID string `json:"id"`
	// Title is the name that a person reads.
	Title string `json:"title"`
	// Body is the text of the post.
	Body string `json:"body"`
}

// PostList is the answer of the list call.
type PostList struct {
	// Posts holds every post, newest first.
	Posts []View `json:"posts"`
}

// view turns one row into the answer of the API. The row of the table and the
// answer therefore change apart.
func view(p *model.Post) View {
	return View{ID: p.ID().String(), Title: p.Title, Body: p.Body}
}

// List answers every post.
//
//avero:response 200 PostList
func (m *Module) List(c *avero.Ctx, in ListInput) (avero.Response, error) {
	posts, err := m.store.List(c.Context(), in.Search)
	if err != nil {
		return nil, err
	}
	out := PostList{Posts: make([]View, 0, len(posts))}
	for _, p := range posts {
		out.Posts = append(out.Posts, view(p))
	}
	return avero.JSON(http.StatusOK, out), nil
}

// Create writes one post.
//
//avero:response 201 View
func (m *Module) Create(c *avero.Ctx, in CreateInput) (avero.Response, error) {
	post, err := m.store.Create(c.Context(), in.Title, in.Body)
	if err != nil {
		return nil, err
	}
	return avero.JSON(http.StatusCreated, view(post)), nil
}

// Show answers one post.
//
//avero:response 200 View
func (m *Module) Show(c *avero.Ctx, in ShowInput) (avero.Response, error) {
	post, found, err := m.store.Get(c.Context(), in.ID)
	if err != nil {
		return nil, err
	}
	if !found {
		return avero.JSON(http.StatusNotFound, map[string]string{"error": "the post is absent"}), nil
	}
	return avero.JSON(http.StatusOK, view(post)), nil
}

// Delete removes one post.
//
//avero:response 204
func (m *Module) Delete(c *avero.Ctx, in DeleteInput) (avero.Response, error) {
	if err := m.store.Delete(c.Context(), in.ID); err != nil {
		return nil, err
	}
	return avero.NoContent(), nil
}
