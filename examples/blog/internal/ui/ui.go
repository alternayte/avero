// Package ui holds the views of the application. It imports no feature
// package, so a view never depends on the code that calls it. See the SDD,
// S10.
package ui

import (
	"context"
	"fmt"
	"html"
	"io"
	"strings"

	"github.com/alternayte/avero"
	"github.com/alternayte/avero/view"
)

// manifest resolves the name of an asset to its hashed path. main.go sets it
// one time at start.
var manifest *avero.Manifest

// SetManifest records the asset manifest of the application.
func SetManifest(m *avero.Manifest) { manifest = m }

// Asset returns the path of a built asset.
//
//	<link rel="stylesheet" href={ ui.Asset("app.css") }>
func Asset(name string) string { return manifest.Asset(name) }

// PostRow is one post that a page renders. The view takes plain values, so it
// needs no import of the feature that reads them.
type PostRow struct {
	// ID identifies the post.
	ID string
	// Title is the name that a person reads.
	Title string
	// Body is the text of the post.
	Body string
}

// Page wraps a body in the layout of the application.
func Page(title string, body avero.ViewComponent) avero.ViewComponent {
	return view.Func(func(ctx context.Context, w io.Writer) error {
		var b strings.Builder
		b.WriteString("<!doctype html>\n<html lang=\"en\">\n<head>\n")
		b.WriteString("<meta charset=\"utf-8\">\n")
		b.WriteString("<meta name=\"viewport\" content=\"width=device-width, initial-scale=1\">\n")
		fmt.Fprintf(&b, "<title>%s · Blog</title>\n", esc(title))
		fmt.Fprintf(&b, "<link rel=\"stylesheet\" href=%q>\n", Asset("app.css"))
		fmt.Fprintf(&b, "<script type=\"module\" src=%q></script>\n", Asset("app.js"))
		b.WriteString("</head>\n<body>\n<header><a href=\"/\">Blog</a></header>\n<main>\n")
		if _, err := io.WriteString(w, b.String()); err != nil {
			return err
		}
		if err := Toasts().Render(ctx, w); err != nil {
			return err
		}
		if err := body.Render(ctx, w); err != nil {
			return err
		}
		_, err := io.WriteString(w, "\n</main>\n</body>\n</html>\n")
		return err
	})
}

// Toasts renders the messages of this response. The flash middleware carries a
// message across a redirect.
func Toasts() avero.ViewComponent {
	return view.Func(func(ctx context.Context, w io.Writer) error {
		toasts := view.Toasts(ctx)
		if len(toasts) == 0 {
			return nil
		}
		var b strings.Builder
		b.WriteString("<ul class=\"toasts\">\n")
		for _, t := range toasts {
			fmt.Fprintf(&b, "<li class=%q>%s</li>\n", esc(t.Level), esc(t.Message))
		}
		b.WriteString("</ul>\n")
		_, err := io.WriteString(w, b.String())
		return err
	})
}

// PostList renders every post.
func PostList(rows []PostRow) avero.ViewComponent {
	return view.Func(func(_ context.Context, w io.Writer) error {
		var b strings.Builder
		b.WriteString("<h1>Posts</h1>\n<p><a href=\"/posts/new\">Write a post</a></p>\n")
		if len(rows) == 0 {
			b.WriteString("<p class=\"empty\">No post exists yet.</p>\n")
		}
		b.WriteString("<ul class=\"posts\">\n")
		for _, row := range rows {
			fmt.Fprintf(&b, "<li><a href=\"/posts/%s\">%s</a></li>\n", esc(row.ID), esc(row.Title))
		}
		b.WriteString("</ul>\n")
		_, err := io.WriteString(w, b.String())
		return err
	})
}

// PostDetail renders one post and the button that deletes it.
func PostDetail(row PostRow) avero.ViewComponent {
	return view.Func(func(ctx context.Context, w io.Writer) error {
		var b strings.Builder
		fmt.Fprintf(&b, "<article><h1>%s</h1><p>%s</p></article>\n", esc(row.Title), esc(row.Body))
		fmt.Fprintf(&b, "<form method=\"post\" action=\"/posts/%s/delete\">\n", esc(row.ID))
		if err := view.CSRF().Render(ctx, &b); err != nil {
			return err
		}
		b.WriteString("\n<button type=\"submit\">Delete</button>\n</form>\n")
		_, err := io.WriteString(w, b.String())
		return err
	})
}

// NewPostForm renders the form of a new post. It renders again after a
// validation fault, with the old input and the field errors. See S10.
func NewPostForm() avero.ViewComponent {
	return view.Func(func(ctx context.Context, w io.Writer) error {
		var b strings.Builder
		b.WriteString("<h1>New post</h1>\n<form method=\"post\" action=\"/posts\">\n")
		if err := view.CSRF().Render(ctx, &b); err != nil {
			return err
		}
		b.WriteString("\n")
		field(&b, ctx, "title", "Title", "input")
		field(&b, ctx, "body", "Body", "textarea")
		b.WriteString("<button type=\"submit\">Save</button>\n</form>\n")
		_, err := io.WriteString(w, b.String())
		return err
	})
}

// NotFound renders the page of an absent post.
func NotFound() avero.ViewComponent {
	return view.Func(func(_ context.Context, w io.Writer) error {
		_, err := io.WriteString(w, "<h1>Absent</h1>\n<p>The post does not exist.</p>\n")
		return err
	})
}

// field renders one form field with its old value and its error.
func field(b *strings.Builder, ctx context.Context, name, label, kind string) {
	fmt.Fprintf(b, "<p><label for=%q>%s</label>\n", name, esc(label))
	old := esc(view.Old(ctx, name))
	if kind == "textarea" {
		fmt.Fprintf(b, "<textarea id=%q name=%q>%s</textarea>\n", name, name, old)
	} else {
		fmt.Fprintf(b, "<input id=%q name=%q value=%q>\n", name, name, old)
	}
	if view.HasError(ctx, name) {
		fmt.Fprintf(b, "<span class=\"error\">%s</span>\n", esc(view.Error(ctx, name)))
	}
	b.WriteString("</p>\n")
}

// esc escapes a value that reaches the page.
func esc(v string) string { return html.EscapeString(v) }
