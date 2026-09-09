package cli_test

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alternayte/avero/assets"
	"github.com/alternayte/avero/internal/cli"
)

// writeFile puts one file into the tree of a test.
func writeFile(t *testing.T, dir, name, body string) {
	t.Helper()
	full := filepath.Join(dir, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatalf("MkdirAll returned %v", err)
	}
	if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
		t.Fatalf("WriteFile returned %v", err)
	}
}

// read returns the body of one file of the tree of a test.
func read(t *testing.T, dir, name string) string {
	t.Helper()
	body, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(name)))
	if err != nil {
		t.Fatalf("ReadFile returned %v", err)
	}
	return string(body)
}

// fakeFetcher answers Fetch from a fixed map of bodies. A test uses it in
// place of the network.
type fakeFetcher struct {
	bodies map[string]string
}

func (f *fakeFetcher) Fetch(_ context.Context, url string) ([]byte, error) {
	body, ok := f.bodies[url]
	if !ok {
		return nil, os.ErrNotExist
	}
	return []byte(body), nil
}

// tarball returns a gzip tarball that holds each named file.
func tarball(t *testing.T, files map[string]string) string {
	t.Helper()
	var buf bytes.Buffer
	zip := gzip.NewWriter(&buf)
	out := tar.NewWriter(zip)
	for name, body := range files {
		if err := out.WriteHeader(&tar.Header{
			Name: name, Mode: 0o644, Size: int64(len(body)), Typeflag: tar.TypeReg,
		}); err != nil {
			t.Fatalf("WriteHeader returned %v", err)
		}
		if _, err := out.Write([]byte(body)); err != nil {
			t.Fatalf("Write returned %v", err)
		}
	}
	if err := out.Close(); err != nil {
		t.Fatalf("Close returned %v", err)
	}
	if err := zip.Close(); err != nil {
		t.Fatalf("Close returned %v", err)
	}
	return buf.String()
}

// fakeBasecoat answers the address of the package with a small tarball of the
// shape that npm publishes.
func fakeBasecoat(t *testing.T) assets.Fetcher {
	t.Helper()
	url := cli.BasecoatURL(assets.BasecoatVersion)
	body := tarball(t, map[string]string{
		"package/dist/basecoat-vega.css": "@layer base {}\n",
		"package/dist/basecoat-nova.css": "@layer base {}\n",
		"package/dist/js/all.min.js":     "(() => {})();\n",
	})
	return &fakeFetcher{bodies: map[string]string{url: body}}
}

// uiProject writes the files that `avero ui add basecoat` reads.
func uiProject(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	writeFile(t, dir, "avero.json", "{\"name\":\"blog\",\"shape\":\"ssr\",\"hypermedia\":\"datastar\"}\n")
	writeFile(t, dir, "assets/css/app.css", "@import \"tailwindcss\";\n\nbody {\n    margin: 0;\n}\n")
	writeFile(t, dir, "assets/js/app.js", "import \"datastar\";\n")
	cli.SetUIFetcher(fakeBasecoat(t))
	t.Cleanup(func() { cli.SetUIFetcher(nil) })
	return dir
}

func TestUIAddBasecoatWritesTheStylesheetTheScriptAndTheComponent(t *testing.T) {
	dir := uiProject(t)

	code, out, errOut := run(t, dir, "ui", "add", "basecoat")
	if code != 0 {
		t.Fatalf("the command gave %d and said %q", code, errOut)
	}

	css := read(t, dir, "assets/css/app.css")
	if !strings.Contains(css, "@import \"../vendor/basecoat/basecoat-vega.css\";") {
		t.Fatalf("the stylesheet is %q", css)
	}
	if strings.Index(css, "tailwindcss") > strings.Index(css, "basecoat-vega") {
		t.Fatal("the Basecoat import stands before the Tailwind import")
	}
	js := read(t, dir, "assets/js/app.js")
	if !strings.Contains(js, "import \"../vendor/basecoat/js/all.min.js\";") {
		t.Fatalf("the script is %q", js)
	}
	if _, err := os.Stat(filepath.Join(dir, "internal", "ui", "toaster.templ")); err != nil {
		t.Fatalf("the component is absent: %v", err)
	}
	if !strings.Contains(out, "basecoat") {
		t.Fatalf("the command said %q", out)
	}
}

func TestUIAddBasecoatRunsTwoTimesWithOneResult(t *testing.T) {
	dir := uiProject(t)

	if code, _, errOut := run(t, dir, "ui", "add", "basecoat"); code != 0 {
		t.Fatalf("the first run gave %d and said %q", code, errOut)
	}
	first := read(t, dir, "assets/css/app.css")
	if code, _, errOut := run(t, dir, "ui", "add", "basecoat"); code != 0 {
		t.Fatalf("the second run gave %d and said %q", code, errOut)
	}
	if second := read(t, dir, "assets/css/app.css"); second != first {
		t.Fatalf("the second run changed the stylesheet to %q", second)
	}
	if strings.Count(read(t, dir, "assets/js/app.js"), "all.min.js") != 1 {
		t.Fatal("the second run added the import again")
	}
}

func TestUIAddBasecoatTakesAStyle(t *testing.T) {
	dir := uiProject(t)

	if code, _, errOut := run(t, dir, "ui", "add", "basecoat", "--style", "nova"); code != 0 {
		t.Fatalf("the command gave %d and said %q", code, errOut)
	}
	if css := read(t, dir, "assets/css/app.css"); !strings.Contains(css, "basecoat-nova.css") {
		t.Fatalf("the stylesheet is %q", css)
	}
}

func TestUIAddBasecoatRefusesAnUnknownStyle(t *testing.T) {
	dir := uiProject(t)

	code, _, errOut := run(t, dir, "ui", "add", "basecoat", "--style", "orion")
	if code != 1 {
		t.Fatalf("the command gave %d, want 1", code)
	}
	if !strings.Contains(errOut, "vega") {
		t.Fatalf("the fault names no known style: %q", errOut)
	}
}

func TestUIAddRefusesAnUnknownLibrary(t *testing.T) {
	dir := uiProject(t)

	code, _, errOut := run(t, dir, "ui", "add", "bootstrap")
	if code != 1 {
		t.Fatalf("the command gave %d, want 1", code)
	}
	if !strings.Contains(errOut, "basecoat") {
		t.Fatalf("the fault names no known library: %q", errOut)
	}
}

func TestUIAddRefusesAShapeThatServesJSON(t *testing.T) {
	dir := uiProject(t)
	writeFile(t, dir, "avero.json", "{\"name\":\"orders\",\"shape\":\"api\"}\n")

	code, _, errOut := run(t, dir, "ui", "add", "basecoat")
	if code != 1 {
		t.Fatalf("the command gave %d, want 1", code)
	}
	if !strings.Contains(errOut, "ssr") {
		t.Fatalf("the fault states no repair: %q", errOut)
	}
}
