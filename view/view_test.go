package view_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/alternayte/avero/config"
	"github.com/alternayte/avero/router"
	"github.com/alternayte/avero/view"
)

// page renders the toasts, the CSRF field, the old input and the field error.
func page(title string) view.Component {
	return view.Func(func(ctx context.Context, w io.Writer) error {
		var b strings.Builder
		fmt.Fprintf(&b, "<h1>%s</h1>", title)
		for _, t := range view.Toasts(ctx) {
			fmt.Fprintf(&b, `<p class="%s">%s</p>`, t.Level, t.Message)
		}
		b.WriteString(`<form method="post">`)
		if err := view.CSRF().Render(ctx, &b); err != nil {
			return err
		}
		fmt.Fprintf(&b, `<input name="title" value="%s">`, view.Old(ctx, "title"))
		if view.HasError(ctx, "title") {
			fmt.Fprintf(&b, `<span class="error">%s</span>`, view.Error(ctx, "title"))
		}
		b.WriteString("</form>")
		_, err := io.WriteString(w, b.String())
		return err
	})
}

func TestViewRendersHTMLWithTwoHundred(t *testing.T) {
	rec := httptest.NewRecorder()
	c := router.NewCtx(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	res := view.View(page("Hello"))
	if res.Status() != http.StatusOK {
		t.Fatalf("Status = %d, want 200", res.Status())
	}
	if err := res.Write(c); err != nil {
		t.Fatalf("Write returned %v, want nil", err)
	}
	if got := rec.Header().Get("Content-Type"); got != "text/html; charset=utf-8" {
		t.Fatalf("Content-Type = %q", got)
	}
	if !strings.Contains(rec.Body.String(), "<h1>Hello</h1>") {
		t.Fatalf("the body holds %q", rec.Body.String())
	}
}

func TestViewStatusUsesTheCode(t *testing.T) {
	rec := httptest.NewRecorder()
	c := router.NewCtx(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	res := view.Status(http.StatusNotFound, page("Absent"))
	if res.Status() != http.StatusNotFound {
		t.Fatalf("Status = %d, want 404", res.Status())
	}
	if err := res.Write(c); err != nil {
		t.Fatalf("Write returned %v, want nil", err)
	}
	if rec.Code != http.StatusNotFound {
		t.Fatalf("the recorder holds %d, want 404", rec.Code)
	}
}

func TestARenderFaultWritesNoBody(t *testing.T) {
	boom := errors.New("the template failed")
	rec := httptest.NewRecorder()
	c := router.NewCtx(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	res := view.View(view.Func(func(_ context.Context, w io.Writer) error {
		_, _ = io.WriteString(w, "half a page")
		return boom
	}))
	err := res.Write(c)
	if !errors.Is(err, boom) {
		t.Fatalf("Write returned %v, want the render fault", err)
	}
	if rec.Body.Len() != 0 {
		t.Fatalf("Write sent %q, want nothing", rec.Body.String())
	}
}

func TestTheHelpersWorkWithAnEmptyContext(t *testing.T) {
	ctx := context.Background()
	if got := view.Old(ctx, "title"); got != "" {
		t.Fatalf("Old = %q, want the empty string", got)
	}
	if got := view.Error(ctx, "title"); got != "" {
		t.Fatalf("Error = %q, want the empty string", got)
	}
	if view.HasError(ctx, "title") {
		t.Fatal("HasError = true, want false")
	}
	if got := view.Toasts(ctx); len(got) != 0 {
		t.Fatalf("Toasts holds %v, want none", got)
	}
	if got := view.Fields(ctx); got != nil {
		t.Fatalf("Fields = %v, want nil", got)
	}
	var b strings.Builder
	if err := view.CSRF().Render(ctx, &b); err != nil {
		t.Fatalf("Render returned %v, want nil", err)
	}
	if strings.Contains(b.String(), "value=\"\"") == false {
		t.Fatalf("the field holds %q, want an empty token", b.String())
	}
}

func TestTheCSRFFieldEscapesTheToken(t *testing.T) {
	ctx := view.WithCSRFToken(context.Background(), `a"<b`)
	var b strings.Builder
	if err := view.CSRF().Render(ctx, &b); err != nil {
		t.Fatalf("Render returned %v, want nil", err)
	}
	if strings.Contains(b.String(), `a"<b`) {
		t.Fatalf("the field holds the raw token: %q", b.String())
	}
	if !strings.Contains(b.String(), `name="`+router.CSRFFieldName+`"`) {
		t.Fatalf("the field holds %q", b.String())
	}
}

func TestOldEscapesNothingAndReturnsTheSubmittedValue(t *testing.T) {
	form := url.Values{"title": {"a title"}}
	ctx := view.WithOld(context.Background(), form)
	if got := view.Old(ctx, "title"); got != "a title" {
		t.Fatalf("Old = %q, want the submitted value", got)
	}
}

func TestToastsKeepTheirOrder(t *testing.T) {
	ctx := view.WithToasts(context.Background(), []router.Toast{
		{Level: router.ToastInfo, Message: "first"},
		{Level: router.ToastError, Message: "second"},
	})
	got := view.Toasts(ctx)
	if len(got) != 2 || got[0].Message != "first" || got[1].Message != "second" {
		t.Fatalf("Toasts holds %v", got)
	}
}

// secret is the signing key of the acceptance tests.
func secret() config.Secret { return config.Secret(strings.Repeat("k", 64)) }

func TestTheHelpersWorkWithANilSessionAndNilFields(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	c := router.NewCtx(rec, req)
	if c.Session() != nil || c.User() != nil {
		t.Fatal("the request carries a session, want none")
	}
	ctx := view.RenderContext(c)
	if view.Fields(ctx) != nil {
		t.Fatalf("Fields = %v, want nil", view.Fields(ctx))
	}
	if view.HasError(ctx, "title") || view.Error(ctx, "title") != "" {
		t.Fatal("the helpers read a field of a request that submitted none")
	}
	if got := view.Old(ctx, "title"); got != "" {
		t.Fatalf("Old = %q, want the empty string", got)
	}
	if got := view.Toasts(ctx); len(got) != 0 {
		t.Fatalf("Toasts holds %v, want none", got)
	}
	nilFields := view.WithFields(context.Background(), nil)
	if view.Error(nilFields, "title") != "" || view.HasError(nilFields, "title") {
		t.Fatal("the helpers read a nil validation result")
	}
	if err := view.View(page("Hello")).Write(c); err != nil {
		t.Fatalf("Write returned %v, want nil", err)
	}
}
