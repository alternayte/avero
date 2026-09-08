package ds_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/alternayte/avero/ds"
	"github.com/alternayte/avero/router"
	"github.com/alternayte/avero/view"
)

// card is a component, as templ writes one.
func card(id, body string) view.Component {
	return view.Func(func(_ context.Context, w io.Writer) error {
		_, err := io.WriteString(w, `<div id="`+id+`">`+body+`</div>`)
		return err
	})
}

// run drives one streaming handler and returns the bytes that the client read.
func run(t *testing.T, fn func(s *ds.Stream) error) string {
	t.Helper()
	rec := httptest.NewRecorder()
	c := router.NewCtx(rec, httptest.NewRequest(http.MethodGet, "/updates", nil))
	res := ds.Open(fn)
	if res.Status() != http.StatusOK {
		t.Fatalf("Status = %d, want 200", res.Status())
	}
	if err := res.Write(c); err != nil {
		t.Fatalf("Write returned %v, want nil", err)
	}
	if got := rec.Header().Get("Content-Type"); got != "text/event-stream" {
		t.Fatalf("Content-Type = %q", got)
	}
	return rec.Body.String()
}

func TestPatchElementsWritesTheDatastarFrame(t *testing.T) {
	got := run(t, func(s *ds.Stream) error { return s.PatchElements(card("a", "x")) })
	want := "event: datastar-patch-elements\ndata: elements <div id=\"a\">x</div>\n\n"
	if got != want {
		t.Fatalf("the stream wrote\n%q\nwant\n%q", got, want)
	}
}

func TestPatchElementsWritesTheSelectorAndTheMode(t *testing.T) {
	got := run(t, func(s *ds.Stream) error {
		return s.PatchElements(card("a", "x"), ds.WithSelector("#list"), ds.WithMode(ds.ModeInner))
	})
	want := "event: datastar-patch-elements\n" +
		"data: selector #list\n" +
		"data: mode inner\n" +
		"data: elements <div id=\"a\">x</div>\n\n"
	if got != want {
		t.Fatalf("the stream wrote\n%q\nwant\n%q", got, want)
	}
}

func TestPatchElementsWritesOneDataLineForEachLineOfMarkup(t *testing.T) {
	got := run(t, func(s *ds.Stream) error {
		return s.PatchHTML("<div id=\"a\">\n  <p>x</p>\n</div>")
	})
	want := "event: datastar-patch-elements\n" +
		"data: elements <div id=\"a\">\n" +
		"data: elements   <p>x</p>\n" +
		"data: elements </div>\n\n"
	if got != want {
		t.Fatalf("the stream wrote\n%q\nwant\n%q", got, want)
	}
}

func TestPatchElementsWritesTheViewTransition(t *testing.T) {
	got := run(t, func(s *ds.Stream) error {
		return s.PatchHTML(`<div id="a"></div>`, ds.WithViewTransition())
	})
	if !strings.Contains(got, "data: useViewTransition true\n") {
		t.Fatalf("the stream wrote %q", got)
	}
}

func TestRemoveElementsWritesTheRemoveMode(t *testing.T) {
	got := run(t, func(s *ds.Stream) error { return s.RemoveElements("#a") })
	want := "event: datastar-patch-elements\ndata: selector #a\ndata: mode remove\n\n"
	if got != want {
		t.Fatalf("the stream wrote\n%q\nwant\n%q", got, want)
	}
}

func TestPatchSignalsWritesTheSignalFrame(t *testing.T) {
	got := run(t, func(s *ds.Stream) error {
		return s.PatchSignals(map[string]any{"count": 2})
	})
	want := "event: datastar-patch-signals\ndata: signals {\"count\":2}\n\n"
	if got != want {
		t.Fatalf("the stream wrote\n%q\nwant\n%q", got, want)
	}
}

func TestPatchSignalsWritesOnlyIfMissing(t *testing.T) {
	got := run(t, func(s *ds.Stream) error {
		return s.PatchSignals(map[string]any{"count": 2}, ds.WithOnlyIfMissing())
	})
	if !strings.Contains(got, "data: onlyIfMissing true\n") {
		t.Fatalf("the stream wrote %q", got)
	}
	if strings.Index(got, "onlyIfMissing") > strings.Index(got, "data: signals") {
		t.Fatalf("onlyIfMissing stands after the signals:\n%s", got)
	}
}

func TestExecuteScriptPatchesTheScriptIntoTheBody(t *testing.T) {
	got := run(t, func(s *ds.Stream) error { return s.ExecuteScript("console.log(1)") })
	want := "event: datastar-patch-elements\n" +
		"data: selector body\n" +
		"data: mode append\n" +
		"data: elements <script data-effect=\"el.remove()\">console.log(1)</script>\n\n"
	if got != want {
		t.Fatalf("the stream wrote\n%q\nwant\n%q", got, want)
	}
}

func TestASecondFrameFollowsTheFirst(t *testing.T) {
	got := run(t, func(s *ds.Stream) error {
		if err := s.PatchHTML(`<p id="a">1</p>`); err != nil {
			return err
		}
		return s.PatchSignals(map[string]any{"n": 1})
	})
	if strings.Count(got, "event: ") != 2 {
		t.Fatalf("the stream wrote %d frames:\n%s", strings.Count(got, "event: "), got)
	}
}

func TestReadSignalsReadsTheQueryOfAGetRequest(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, `/updates?datastar={"count":7}`, nil)
	c := router.NewCtx(httptest.NewRecorder(), req)
	var in struct {
		Count int `json:"count"`
	}
	if err := ds.ReadSignals(c, &in); err != nil {
		t.Fatalf("ReadSignals returned %v, want nil", err)
	}
	if in.Count != 7 {
		t.Fatalf("Count = %d, want 7", in.Count)
	}
}

func TestReadSignalsReadsTheBodyOfAPostRequest(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/updates", strings.NewReader(`{"count":9}`))
	c := router.NewCtx(httptest.NewRecorder(), req)
	var in struct {
		Count int `json:"count"`
	}
	if err := ds.ReadSignals(c, &in); err != nil {
		t.Fatalf("ReadSignals returned %v, want nil", err)
	}
	if in.Count != 9 {
		t.Fatalf("Count = %d, want 9", in.Count)
	}
}

func TestReadSignalsNamesTheRepairForABodyThatIsNotJSON(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/updates", strings.NewReader("count=9"))
	c := router.NewCtx(httptest.NewRecorder(), req)
	var in struct{}
	err := ds.ReadSignals(c, &in)
	if err == nil {
		t.Fatal("ReadSignals returned nil, want a fault")
	}
	if !strings.Contains(err.Error(), "→") {
		t.Fatalf("the fault states no repair: %v", err)
	}
}

func TestPartialReportsADatastarRequest(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/updates", nil)
	req.Header.Set("Datastar-Request", "true")
	c := router.NewCtx(httptest.NewRecorder(), req)
	if !ds.Partial(c) {
		t.Fatal("Partial = false, want true")
	}
	plain := router.NewCtx(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))
	if ds.Partial(plain) {
		t.Fatal("Partial = true for a plain request")
	}
}

// blocker is a writer that reports a flush and blocks nothing.
func TestADisconnectClosesTheStreamAndReleasesTheGoroutine(t *testing.T) {
	before := goroutines()

	ctx, cancel := context.WithCancel(context.Background())
	req := httptest.NewRequest(http.MethodGet, "/updates", nil).WithContext(ctx)
	rec := httptest.NewRecorder()
	c := router.NewCtx(rec, req)

	started := make(chan struct{})
	done := make(chan error, 1)
	go func() {
		done <- ds.Open(func(s *ds.Stream) error {
			close(started)
			for {
				select {
				case <-s.Done():
					// The client left. The handler returns.
					return nil
				case <-time.After(time.Millisecond):
					if err := s.PatchHTML(`<p id="a">tick</p>`); err != nil {
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
	ctx, cancel := context.WithCancel(context.Background())
	req := httptest.NewRequest(http.MethodGet, "/updates", nil).WithContext(ctx)
	rec := httptest.NewRecorder()
	cancel()

	err := ds.Open(func(s *ds.Stream) error {
		return s.PatchHTML(`<p id="a">x</p>`)
	}).Write(router.NewCtx(rec, req))
	if err != nil {
		t.Fatalf("Write returned %v, want nil", err)
	}
	if strings.Contains(rec.Body.String(), "elements") {
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
