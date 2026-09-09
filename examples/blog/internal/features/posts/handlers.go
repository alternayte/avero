package posts

import (
	"net/http"

	"github.com/alternayte/avero"
	"github.com/alternayte/avero/ds"

	"blog/internal/features/posts/model"
	"blog/internal/ui"
)

// List renders every post.
func (m *Module) List(c *avero.Ctx, in ListInput) (avero.Response, error) {
	posts, err := m.store.List(c, in.Search)
	if err != nil {
		return nil, err
	}
	rows := make([]ui.PostRow, 0, len(posts))
	for _, p := range posts {
		rows = append(rows, row(p))
	}
	return avero.View(ui.Page("Posts", ui.PostList(rows))), nil
}

// New renders the empty form.
func (m *Module) New(c *avero.Ctx, _ NewInput) (avero.Response, error) {
	return avero.View(ui.Page("New post", ui.NewPostForm())), nil
}

// Create writes one post and sends the person back to the list.
func (m *Module) Create(c *avero.Ctx, in CreateInput) (avero.Response, error) {
	if _, err := m.store.Create(c, in.Title, in.Body); err != nil {
		return nil, err
	}
	c.Success("The post is saved")
	return avero.Redirect(http.StatusSeeOther, "/"), nil
}

// Show renders one post.
func (m *Module) Show(c *avero.Ctx, in ShowInput) (avero.Response, error) {
	post, found, err := m.store.Get(c, in.ID)
	if err != nil {
		return nil, err
	}
	if !found {
		return avero.ViewStatus(http.StatusNotFound, ui.Page("Absent", ui.NotFound())), nil
	}
	return avero.View(ui.Page(post.Title, ui.PostDetail(row(post)))), nil
}

// Delete removes one post.
//
// Datastar sends the request, so the answer is a patch and not a page. The
// browser removes the row and keeps everything else. See S12.
func (m *Module) Delete(c *avero.Ctx, in DeleteInput) (avero.Response, error) {
	if err := m.store.Delete(c, in.ID); err != nil {
		return nil, err
	}
	posts, err := m.store.List(c, "")
	if err != nil {
		return nil, err
	}
	return ds.Open(func(s *ds.Stream) error {
		if err := s.RemoveElements("#post-" + in.ID); err != nil {
			return err
		}
		if len(posts) > 0 {
			return nil
		}
		// The list is empty again, so the page states it.
		return s.PatchHTML(`<p id="posts-empty" class="empty">No post exists yet.</p>`)
	}), nil
}

// row maps one post to the shape that a view reads.
func row(p *model.Post) ui.PostRow {
	return ui.PostRow{ID: p.ID().String(), Title: p.Title, Body: p.Body}
}
