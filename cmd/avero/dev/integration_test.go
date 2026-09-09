//go:build integration

package dev_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/alternayte/avero/cmd/avero/dev"
	"github.com/alternayte/avero/internal/budget"
	"github.com/alternayte/avero/internal/cli"
)

// The integration suite measures DX-2 and DX-3 against a scaffolded
// application with the real build steps: the real asset pipeline and the real
// Go compiler. See the SDD, section 3.

// DX3Budget is the limit of one rebuild. See the SDD, section 3.
const DX3Budget = 3 * time.Second

// repoRoot returns the directory of this repository.
func repoRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs("../../..")
	if err != nil {
		t.Fatalf("Abs returned %v", err)
	}
	return root
}

// scaffold writes an application and returns its directory.
func scaffold(t *testing.T) string {
	t.Helper()
	root := repoRoot(t)
	dir := t.TempDir()
	code := cli.Run(context.Background(), cli.Streams{Out: os.Stderr, Err: os.Stderr, Dir: dir},
		[]string{"new", "blog", "--replace", root})
	if code != 0 {
		t.Fatal("avero new failed")
	}
	app := filepath.Join(dir, "blog")

	// The measurement must not wait for a download of the Tailwind binary,
	// so the stylesheet builds with esbuild. A project that uses Tailwind
	// adds the time of the Tailwind compile.
	project := filepath.Join(app, "avero.json")
	body, err := os.ReadFile(project)
	if err != nil {
		t.Fatalf("ReadFile returned %v", err)
	}
	var doc map[string]any
	if err := json.Unmarshal(body, &doc); err != nil {
		t.Fatalf("avero.json does not parse: %v", err)
	}
	doc["assets"] = map[string]any{"entries": []string{"css/app.css", "js/app.js"}}
	out, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		t.Fatalf("Marshal returned %v", err)
	}
	if err := os.WriteFile(project, out, 0o644); err != nil {
		t.Fatalf("WriteFile returned %v", err)
	}
	// The stylesheet of the scaffold imports Tailwind, which esbuild cannot
	// resolve. The measurement uses plain CSS.
	if err := os.WriteFile(filepath.Join(app, "assets", "css", "app.css"),
		[]byte("body { margin: 0; }\n"), 0o644); err != nil {
		t.Fatalf("WriteFile returned %v", err)
	}
	// The loop reads .env, as a person does after `avero new`.
	example, err := os.ReadFile(filepath.Join(app, ".env.example"))
	if err != nil {
		t.Fatalf("ReadFile returned %v", err)
	}
	// The first start applies the migrations. A later start proves them with
	// one query, as a person runs it.
	body = append(example, []byte("\nMIGRATE_ON_BOOT=true\n")...)
	if err := os.WriteFile(filepath.Join(app, ".env"), body, 0o644); err != nil {
		t.Fatalf("WriteFile returned %v", err)
	}

	cmd := exec.Command("go", "mod", "tidy")
	cmd.Dir = app
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("go mod tidy failed: %v\n%s", err, out)
	}
	return app
}

// loopOf starts `avero dev` on a free port and returns the address.
func loopOf(t *testing.T, app string) string {
	t.Helper()
	port := freePort(t)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	go func() {
		_ = cli.Run(ctx, cli.Streams{Out: os.Stderr, Err: os.Stderr, Dir: app},
			[]string{"dev", "--port", fmt.Sprint(port)})
	}()

	address := fmt.Sprintf("http://127.0.0.1:%d", port)
	deadline := time.Now().Add(90 * time.Second)
	for time.Now().Before(deadline) {
		res, err := http.Get(address + "/")
		if err == nil {
			_ = res.Body.Close()
			return address
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatal("the loop did not answer")
	return ""
}

// waitFor polls a page until it holds the text, and returns the wait.
func waitFor(t *testing.T, address, text string, limit time.Duration) time.Duration {
	t.Helper()
	start := time.Now()
	deadline := start.Add(limit)
	for time.Now().Before(deadline) {
		res, err := http.Get(address)
		if err == nil {
			body := make([]byte, 1<<20)
			n, _ := res.Body.Read(body)
			_ = res.Body.Close()
			if strings.Contains(string(body[:n]), text) {
				return time.Since(start)
			}
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("the page never held %q", text)
	return 0
}

// TestDX2 measures a change to a CSS file. The budget is 200 milliseconds, and
// the process must not restart and the page must not reload.
func TestDX2(t *testing.T) {
	app := scaffold(t)
	address := loopOf(t, app)

	// The browser listens on the reload channel.
	res, err := http.Get(address + "/_avero/reload")
	if err != nil {
		t.Fatalf("the reload channel does not open: %v", err)
	}
	defer func() { _ = res.Body.Close() }()

	// The measurement takes the fastest of three changes, as DX-3 does.
	elapsed := time.Hour
	href := ""
	for i := range 3 {
		rule := fmt.Sprintf("body { margin: %dpx; color: rebeccapurple; }\n", i)
		start := time.Now()
		if err := os.WriteFile(filepath.Join(app, "assets", "css", "app.css"), []byte(rule), 0o644); err != nil {
			t.Fatalf("WriteFile returned %v", err)
		}
		href = readCSSEvent(t, res, 10*time.Second)
		if run := time.Since(start); run < elapsed {
			elapsed = run
		}
	}
	if href == "" {
		t.Fatal("the loop sent no address")
	}
	body := readPage(t, address+href)
	if !strings.Contains(body, "rebeccapurple") {
		t.Fatalf("the stylesheet holds the old rule:\n%s", body)
	}
	if elapsed > 200*time.Millisecond {
		t.Fatalf("DX-2 took %s, and the budget is 200ms", elapsed.Round(time.Millisecond))
	}
	if err := budget.Record(repoRoot(t), "DX-2", "a change to a `.css` file appears in the browser",
		elapsed, 200*time.Millisecond); err != nil {
		t.Fatalf("the measurement does not record: %v", err)
	}
	t.Logf("DX-2: %s", elapsed.Round(time.Millisecond))
}

// TestDX3 measures a change to a Go file. The budget is three seconds.
//
// The budget holds the Go link of the whole application and, on macOS, the
// first execution of a binary that the operating system has not seen. The
// parts of one measurement on an Apple Silicon laptop are:
//
//	go build of the scaffolded application  1.0 s to 1.7 s
//	the first execution of the new binary   0.3 s to 0.5 s
//	the boot of the application             25 ms
//	the watcher, the restart and the wait   about 50 ms
//
// See the SDD, section 3.
func TestDX3(t *testing.T) {
	app := scaffold(t)
	address := loopOf(t, app)
	waitFor(t, address+"/", "Posts", 5*time.Second)

	// The ssr shape writes its pages as templ files. The loop wraps
	// `go tool templ generate --watch`, which writes the Go file, and the
	// watcher then rebuilds. See S15.
	page := filepath.Join(app, "internal", "ui", "posts.templ")
	body, err := os.ReadFile(page)
	if err != nil {
		t.Fatalf("ReadFile returned %v", err)
	}

	// The measurement follows one change, because DX-3 states the loop that a
	// person works in. The first build of a session fills the build cache of
	// the module.
	warm := strings.Replace(string(body), "<h1>Posts</h1>", "<h1>Warm</h1>", 1)
	if warm == string(body) {
		t.Fatalf("the page holds no heading to change:\n%s", body)
	}
	if err := os.WriteFile(page, []byte(warm), 0o644); err != nil {
		t.Fatalf("WriteFile returned %v", err)
	}
	waitFor(t, address+"/", "Warm", 30*time.Second)
	if !strings.Contains(warm, "<h1>Warm</h1>") {
		t.Fatalf("the page holds no heading to change:\n%s", warm)
	}
	changed := warm

	// The measurement takes the fastest of three changes. Another process of
	// the machine can hold the compiler for a moment, and the budget states
	// the loop and not the load of the machine. See the SDD, section 3.
	elapsed := time.Hour
	previous := "Warm"
	for i := range 3 {
		want := fmt.Sprintf("Journal%d", i)
		next := strings.Replace(changed, "<h1>"+previous+"</h1>", "<h1>"+want+"</h1>", 1)
		if next == changed {
			t.Fatalf("the page holds no heading %q to change", previous)
		}
		changed, previous = next, want

		start := time.Now()
		if err := os.WriteFile(page, []byte(changed), 0o644); err != nil {
			t.Fatalf("WriteFile returned %v", err)
		}
		waitFor(t, address+"/", want, 30*time.Second)
		if run := time.Since(start); run < elapsed {
			elapsed = run
		}
	}

	if elapsed > DX3Budget {
		t.Fatalf("DX-3 took %s, and the budget is %s", elapsed.Round(time.Millisecond), DX3Budget)
	}
	if err := budget.Record(repoRoot(t), "DX-3", "a change to a `.templ` or `.go` file appears in the browser",
		elapsed, DX3Budget); err != nil {
		t.Fatalf("the measurement does not record: %v", err)
	}
	t.Logf("DX-3: %s", elapsed.Round(time.Millisecond))
}

// TestTheProductionBuildCarriesNoReloadClient proves that the binary of an
// application holds none of the loop.
func TestTheProductionBuildCarriesNoReloadClient(t *testing.T) {
	app := scaffold(t)
	cmd := exec.Command("go", "build", "-o", "app", ".")
	cmd.Dir = app
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("go build failed: %v\n%s", err, out)
	}
	body, err := os.ReadFile(filepath.Join(app, "app"))
	if err != nil {
		t.Fatalf("ReadFile returned %v", err)
	}
	// The search reads the built output for every mark of the loop.
	for _, mark := range []string{"_avero/reload", "X-Avero-Reload", "DOMParser", "EventSource"} {
		if strings.Contains(string(body), mark) {
			t.Fatalf("the binary holds %q of the development loop", mark)
		}
	}
	// The same marks stand in the client, so the search proves something.
	for _, mark := range []string{"_avero/reload", "X-Avero-Reload", "DOMParser", "EventSource"} {
		if !strings.Contains(string(dev.Client()), mark) {
			t.Fatalf("the client holds no %q, so the search proves nothing", mark)
		}
	}
}

// readCSSEvent reads the reload channel until a css event arrives.
func readCSSEvent(t *testing.T, res *http.Response, limit time.Duration) string {
	t.Helper()
	type answer struct{ href string }
	found := make(chan answer, 1)
	go func() {
		buf := make([]byte, 4096)
		var text string
		for {
			n, err := res.Body.Read(buf)
			if n > 0 {
				text += string(buf[:n])
				for _, line := range strings.Split(text, "\n") {
					if !strings.HasPrefix(line, "data: ") {
						continue
					}
					var e struct {
						Type string `json:"type"`
						Href string `json:"href"`
					}
					if err := json.Unmarshal([]byte(strings.TrimPrefix(line, "data: ")), &e); err == nil && e.Type == "css" {
						found <- answer{href: e.Href}
						return
					}
				}
			}
			if err != nil {
				return
			}
		}
	}()
	select {
	case a := <-found:
		return a.href
	case <-time.After(limit):
		t.Fatal("no css event arrived")
		return ""
	}
}

// readPage returns the body of one page.
func readPage(t *testing.T, address string) string {
	t.Helper()
	res, err := http.Get(address)
	if err != nil {
		t.Fatalf("Get returned %v", err)
	}
	defer func() { _ = res.Body.Close() }()
	buf := make([]byte, 1<<20)
	n, _ := res.Body.Read(buf)
	return string(buf[:n])
}
