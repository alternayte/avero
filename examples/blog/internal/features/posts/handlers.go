package posts

import (
	"net/http"

	"github.com/alternayte/avero"

	"blog/internal/ui"
)

// List renders every post.
func (m *Module) List(c *avero.Ctx, in ListInput) (avero.Response, error) {
	posts, err := m.store.List(c.Context(), in.Search)
	if err != nil {
		return nil, err
	}
	rows := make([]ui.PostRow, 0, len(posts))
	for _, p := range posts {
		rows = append(rows, ui.PostRow{ID: p.ID, Title: p.Title, Body: p.Body})
	}
	return avero.View(ui.Page("Posts", ui.PostList(rows))), nil
}

// New renders the empty form.
func (m *Module) New(c *avero.Ctx, _ NewInput) (avero.Response, error) {
	return avero.View(ui.Page("New post", ui.NewPostForm())), nil
}

// Create writes one post and sends the person back to the list.
func (m *Module) Create(c *avero.Ctx, in CreateInput) (avero.Response, error) {
	if _, err := m.store.Create(c.Context(), in.Title, in.Body); err != nil {
		return nil, err
	}
	c.Success("The post is saved")
	return avero.Redirect(http.StatusSeeOther, "/"), nil
}

// Show renders one post.
func (m *Module) Show(c *avero.Ctx, in ShowInput) (avero.Response, error) {
	post, found, err := m.store.Get(c.Context(), in.ID)
	if err != nil {
		return nil, err
	}
	if !found {
		return avero.ViewStatus(http.StatusNotFound, ui.Page("Absent", ui.NotFound())), nil
	}
	return avero.View(ui.Page(post.Title, ui.PostDetail(ui.PostRow{
		ID: post.ID, Title: post.Title, Body: post.Body,
	}))), nil
}

// Delete removes one post.
func (m *Module) Delete(c *avero.Ctx, in DeleteInput) (avero.Response, error) {
	if err := m.store.Delete(c.Context(), in.ID); err != nil {
		return nil, err
	}
	c.Info("The post is deleted")
	return avero.Redirect(http.StatusSeeOther, "/"), nil
}
