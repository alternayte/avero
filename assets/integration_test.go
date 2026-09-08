//go:build integration

package assets_test

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/alternayte/avero/assets"
)

// nanoStores is a bundled ES module on a public CDN. `avero js pin` fetches
// exactly this shape.
const nanoStores = "https://cdn.jsdelivr.net/npm/nanostores@0.11.3/+esm"

// serverMain is the application that proves the embedded file system. It holds
// the two lines that a scaffolded main.go holds for the assets.
const serverMain = `package main

import (
	"embed"
	"fmt"
	"net/http"
	"os"

	"github.com/alternayte/avero/assets"
)

//go:embed all:assets/dist
var dist embed.FS

func main() {
	m, err := assets.LoadManifest(dist, "assets/dist/manifest.json")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	sub, err := fsSub()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	mux := http.NewServeMux()
	mux.Handle("/assets/", http.StripPrefix("/assets/", assets.Handler(sub, m)))
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "<link rel=%q href=%q>", "stylesheet", m.Asset("app.css"))
	})
	fmt.Println("ready")
	_ = http.Serve(listener(), mux)
}
`

// serverHelpers holds the two functions that keep the main function short.
const serverHelpers = `package main

import (
	"fmt"
	"io/fs"
	"net"
	"os"
)

func fsSub() (fs.FS, error) { return fs.Sub(dist, "assets/dist") }

func listener() net.Listener {
	l, err := net.Listen("tcp", "127.0.0.1:"+os.Getenv("PORT"))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	return l
}
`

// TestAScaffoldedApplicationBuildsWithNoNodeAndServesTheAssets drives the
// whole pipeline: it pins a module from a CDN, downloads the Tailwind
// standalone binary, builds tier 0 with no Node.js on the path, compiles a
// binary that embeds the output, and reads one asset over HTTP.
func TestAScaffoldedApplicationBuildsWithNoNodeAndServesTheAssets(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the test builds a POSIX application")
	}
	root, err := filepath.Abs("..")
	if err != nil {
		t.Fatalf("Abs returned %v", err)
	}
	dir := t.TempDir()
	write(t, dir, "assets/css/app.css", "@import \"tailwindcss\";\nh1 { color: red; }\n")
	write(t, dir, "assets/js/app.js", "import { atom } from \"nanostores\";\nwindow.count = atom(0);\n")
	write(t, dir, "main.go", serverMain)
	write(t, dir, "helpers.go", serverHelpers)
	write(t, dir, "go.mod", "module scaffolded\n\ngo 1.26.2\n\nrequire github.com/alternayte/avero v0.0.0\n\nreplace github.com/alternayte/avero => "+root+"\n")

	ctx := context.Background()
	if err := assets.PinJS(ctx, assets.PinConfig{Dir: dir}, "nanostores", nanoStores); err != nil {
		t.Fatalf("PinJS returned %v, want nil", err)
	}

	// DX-9. The build runs with an empty path, so no Node.js and no other
	// tool can reach it. The Tailwind binary runs from .avero/bin.
	t.Setenv("PATH", "")
	m, err := assets.Build(ctx, assets.Config{
		Dir:      dir,
		Entries:  []string{"js/app.js"},
		Minify:   true,
		Tailwind: &assets.Tailwind{Dir: dir, Input: "assets/css/app.css", Minify: true},
	})
	if err != nil {
		t.Fatalf("Build returned %v, want nil", err)
	}
	entry, ok := m.Entry("app.css")
	if !ok {
		t.Fatalf("the manifest holds no stylesheet, it holds %v", m.Names())
	}
	if got := m.Asset("app.css"); got != "/assets/"+entry.File {
		t.Fatalf("Asset = %q, want the hashed path", got)
	}
	css, err := os.ReadFile(filepath.Join(dir, "assets/dist", entry.File))
	if err != nil {
		t.Fatalf("ReadFile returned %v", err)
	}
	if !strings.Contains(string(css), "color:red") {
		t.Fatalf("Tailwind emitted no rule of the person:\n%s", firstBytes(css))
	}

	// The binary embeds the output directory and serves it.
	t.Setenv("PATH", os.Getenv("HOME")+"/go/bin:/usr/local/go/bin:/opt/homebrew/bin:/usr/bin:/bin")
	for _, args := range [][]string{{"mod", "tidy"}, {"build", "-o", "app", "."}} {
		cmd := exec.CommandContext(ctx, "go", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("go %s failed: %v\n%s", strings.Join(args, " "), err, out)
		}
	}

	port := freePort(t)
	run := exec.CommandContext(ctx, "./app")
	run.Dir = dir
	run.Env = append(os.Environ(), "PORT="+port)
	if err := run.Start(); err != nil {
		t.Fatalf("the application does not start: %v", err)
	}
	defer func() { _ = run.Process.Kill() }()
	waitFor(t, "http://127.0.0.1:"+port+"/")

	res, err := http.Get("http://127.0.0.1:" + port + m.Asset("app.css"))
	if err != nil {
		t.Fatalf("Get returned %v", err)
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", res.StatusCode)
	}
	if got := res.Header.Get("Cache-Control"); got != assets.CacheControl {
		t.Fatalf("Cache-Control = %q", got)
	}

	absent, err := http.Get("http://127.0.0.1:" + port + "/assets/app.css")
	if err != nil {
		t.Fatalf("Get returned %v", err)
	}
	defer func() { _ = absent.Body.Close() }()
	if absent.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want 404 for a name outside the manifest", absent.StatusCode)
	}
}

// firstBytes returns the head of a body, for a message.
func firstBytes(b []byte) string {
	if len(b) > 400 {
		return string(b[:400])
	}
	return string(b)
}

// freePort returns a port that no process holds.
func freePort(t *testing.T) string {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen returned %v", err)
	}
	defer func() { _ = l.Close() }()
	return fmt.Sprint(l.Addr().(*net.TCPAddr).Port)
}

// waitFor blocks until the application answers.
func waitFor(t *testing.T, url string) {
	t.Helper()
	for range 100 {
		res, err := http.Get(url)
		if err == nil {
			_ = res.Body.Close()
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatal("the application did not answer in five seconds")
}
