package htmx_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/alternayte/avero/htmx"
	"github.com/alternayte/avero/router"
	"github.com/alternayte/avero/view"
)

// row is a component, as templ writes one.
func row(id, body string) view.Component {
	return view.Func(func(_ context.Context, w io.Writer) error {
		_, err := io.WriteString(w, `<li id="`+id+`">`+body+`</li>`)
		return err
	})
}

// ctx returns a request context and its recorder.
func ctx(t *testing.T, headers map[string]string) (*router.Ctx, *httptest.ResponseRecorder) {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/rows", nil)
	for name, value := range headers {
		req.Header.Set(name, value)
	}
	rec := httptest.NewRecorder()
	return router.NewCtx(rec, req), rec
}

func TestPartialReportsAnHtmxRequest(t *testing.T) {
	c, _ := ctx(t, map[string]string{htmx.RequestHeader: "true"})
	if !htmx.Partial(c) {
		t.Fatal("Partial = false, want true")
	}
	plain, _ := ctx(t, nil)
	if htmx.Partial(plain) {
		t.Fatal("Partial = true for a plain request")
	}
}

func TestTheRequestHelpersReadTheHeaders(t *testing.T) {
	c, _ := ctx(t, map[string]string{
		htmx.RequestHeader:           "true",
		"HX-Boosted":                 "true",
		"HX-Target":                  "rows",
		"HX-Trigger":                 "add",
		"HX-Trigger-Name":            "title",
		"HX-Current-URL":             "https://example.com/rows",
		"HX-Prompt":                  "a name",
		"HX-History-Restore-Request": "true",
	})
	if !htmx.Boosted(c) || !htmx.HistoryRestore(c) {
		t.Fatal("Boosted or HistoryRestore reads false, want true")
	}
	if htmx.Target(c) != "rows" || htmx.TriggerID(c) != "add" || htmx.TriggerName(c) != "title" {
		t.Fatalf("the helpers read %q %q %q", htmx.Target(c), htmx.TriggerID(c), htmx.TriggerName(c))
	}
	if htmx.CurrentURL(c) != "https://example.com/rows" || htmx.Prompt(c) != "a name" {
		t.Fatalf("the helpers read %q %q", htmx.CurrentURL(c), htmx.Prompt(c))
	}
}

func TestFragmentWritesTheMarkupOfEveryComponent(t *testing.T) {
	c, rec := ctx(t, nil)
	res := htmx.Fragment(row("a", "1"), row("b", "2"))
	if res.Status() != http.StatusOK {
		t.Fatalf("Status = %d, want 200", res.Status())
	}
	if err := res.Write(c); err != nil {
		t.Fatalf("Write returned %v, want nil", err)
	}
	if got := rec.Header().Get("Content-Type"); got != "text/html; charset=utf-8" {
		t.Fatalf("Content-Type = %q", got)
	}
	if rec.Body.String() != `<li id="a">1</li><li id="b">2</li>` {
		t.Fatalf("the body holds %q", rec.Body.String())
	}
}

func TestFragmentStatusUsesTheCode(t *testing.T) {
	c, rec := ctx(t, nil)
	if err := htmx.FragmentStatus(http.StatusUnprocessableEntity, row("a", "1")).Write(c); err != nil {
		t.Fatalf("Write returned %v, want nil", err)
	}
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422", rec.Code)
	}
}

func TestTheResponseHelpersWriteTheHeaders(t *testing.T) {
	c, rec := ctx(t, nil)
	htmx.Retarget(c, "#rows")
	htmx.Reswap(c, "beforeend")
	htmx.Reselect(c, "#row")
	htmx.PushURL(c, "/rows/2")
	htmx.ReplaceURL(c, "/rows/3")
	htmx.Refresh(c)
	if err := htmx.Fragment(row("a", "1")).Write(c); err != nil {
		t.Fatalf("Write returned %v, want nil", err)
	}
	want := map[string]string{
		"HX-Retarget":    "#rows",
		"HX-Reswap":      "beforeend",
		"HX-Reselect":    "#row",
		"HX-Push-Url":    "/rows/2",
		"HX-Replace-Url": "/rows/3",
		"HX-Refresh":     "true",
	}
	for name, value := range want {
		if got := rec.Header().Get(name); got != value {
			t.Fatalf("%s = %q, want %q", name, got, value)
		}
	}
}

func TestTriggerEncodesTheDetail(t *testing.T) {
	c, rec := ctx(t, nil)
	if err := htmx.Trigger(c, map[string]any{"saved": map[string]any{"id": 7}}); err != nil {
		t.Fatalf("Trigger returned %v, want nil", err)
	}
	if err := htmx.TriggerAfterSwap(c, "closed"); err != nil {
		t.Fatalf("TriggerAfterSwap returned %v, want nil", err)
	}
	if err := htmx.Fragment(row("a", "1")).Write(c); err != nil {
		t.Fatalf("Write returned %v, want nil", err)
	}
	if got := rec.Header().Get("HX-Trigger"); got != `{"saved":{"id":7}}` {
		t.Fatalf("HX-Trigger = %q", got)
	}
	if got := rec.Header().Get("HX-Trigger-After-Swap"); got != "closed" {
		t.Fatalf("HX-Trigger-After-Swap = %q", got)
	}
}

func TestRedirectAndLocationWriteTheHeaders(t *testing.T) {
	c, rec := ctx(t, nil)
	res := htmx.Redirect(c, "/rows")
	if res.Status() != http.StatusOK {
		t.Fatalf("Status = %d, want 200", res.Status())
	}
	if err := res.Write(c); err != nil {
		t.Fatalf("Write returned %v, want nil", err)
	}
	if got := rec.Header().Get("HX-Redirect"); got != "/rows" {
		t.Fatalf("HX-Redirect = %q", got)
	}

	other, otherRec := ctx(t, nil)
	if err := htmx.Location(other, "/rows").Write(other); err != nil {
		t.Fatalf("Write returned %v, want nil", err)
	}
	if got := otherRec.Header().Get("HX-Location"); got != "/rows" {
		t.Fatalf("HX-Location = %q", got)
	}
}

func TestTheStreamWritesOneEventForEachFragment(t *testing.T) {
	c, rec := ctx(t, nil)
	res := htmx.Open(func(s *htmx.Stream) error {
		if err := s.Send("row", row("a", "1")); err != nil {
			return err
		}
		return s.SendHTML("row", "<li id=\"b\">\n2</li>")
	})
	if err := res.Write(c); err != nil {
		t.Fatalf("Write returned %v, want nil", err)
	}
	want := "event: row\ndata: <li id=\"a\">1</li>\n\n" +
		"event: row\ndata: <li id=\"b\">\ndata: 2</li>\n\n"
	if rec.Body.String() != want {
		t.Fatalf("the stream wrote\n%q\nwant\n%q", rec.Body.String(), want)
	}
	if got := rec.Header().Get("Content-Type"); got != "text/event-stream" {
		t.Fatalf("Content-Type = %q", got)
	}
}

func TestADisconnectClosesTheStreamAndReleasesTheGoroutine(t *testing.T) {
	before := goroutines()

	cancelCtx, cancel := context.WithCancel(context.Background())
	req := httptest.NewRequest(http.MethodGet, "/rows", nil).WithContext(cancelCtx)
	c := router.NewCtx(httptest.NewRecorder(), req)

	started := make(chan struct{})
	done := make(chan error, 1)
	go func() {
		done <- htmx.Open(func(s *htmx.Stream) error {
			close(started)
			for {
				select {
				case <-s.Done():
					return nil
				case <-time.After(time.Millisecond):
					if err := s.SendHTML("row", `<li id="a">tick</li>`); err != nil {
						return err
					}
				}
			}
		}).Write(c)
	}()

	<-started
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Write returned %v, want nil", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("the stream did not close after the disconnect")
	}
	if leaked := goroutines() - before; leaked > 0 {
		t.Fatalf("%d goroutines stayed behind", leaked)
	}
}

func TestAFrameAfterADisconnectReturnsTheClosedFault(t *testing.T) {
	cancelCtx, cancel := context.WithCancel(context.Background())
	req := httptest.NewRequest(http.MethodGet, "/rows", nil).WithContext(cancelCtx)
	rec := httptest.NewRecorder()
	cancel()

	err := htmx.Open(func(s *htmx.Stream) error {
		return s.SendHTML("row", "<li></li>")
	}).Write(router.NewCtx(rec, req))
	if err != nil {
		t.Fatalf("Write returned %v, want nil", err)
	}
	if strings.Contains(rec.Body.String(), "<li>") {
		t.Fatalf("the stream wrote a frame after the disconnect:\n%q", rec.Body.String())
	}
}

// goroutines returns the number of goroutines after the runtime settles.
func goroutines() int {
	for range 20 {
		runtime.GC()
		time.Sleep(10 * time.Millisecond)
	}
	return runtime.NumGoroutine()
}
