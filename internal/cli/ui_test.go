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
	"github.com/alternayte/avero/scaffold"
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

// uiProject writes the files that `avero ui add basecoat` reads. It returns
// the directory and the fetcher that a test passes on `cli.Streams`.
func uiProject(t *testing.T) (string, assets.Fetcher) {
	t.Helper()
	dir := t.TempDir()
	writeFile(t, dir, "avero.json", "{\"name\":\"blog\",\"shape\":\"ssr\",\"hypermedia\":\"datastar\"}\n")
	writeFile(t, dir, "assets/css/app.css", "@import \"tailwindcss\";\n\nbody {\n    margin: 0;\n}\n")
	writeFile(t, dir, "assets/js/app.js", "import \"datastar\";\n")
	return dir, fakeBasecoat(t)
}

// runUI performs one command with the fetcher of a ui test.
func runUI(t *testing.T, dir string, fetch assets.Fetcher, args ...string) (int, string, string) {
	t.Helper()
	var out, errOut bytes.Buffer
	code := cli.Run(context.Background(), cli.Streams{Out: &out, Err: &errOut, Dir: dir, Fetch: fetch}, args)
	return code, out.String(), errOut.String()
}

func TestUIAddBasecoatWritesTheStylesheetTheScriptAndTheComponent(t *testing.T) {
	dir, fetch := uiProject(t)

	code, out, errOut := runUI(t, dir, fetch, "ui", "add", "basecoat")
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
	if _, err := os.Stat(filepath.Join(dir, "toaster_test.go")); err != nil {
		t.Fatalf("the proof is absent: %v", err)
	}
	if !strings.Contains(out, "basecoat") {
		t.Fatalf("the command said %q", out)
	}
}

// TestUIAddBasecoatWritesTheProofOfTheToaster proves that the command puts
// the marks of Basecoat into toaster_test.go.
func TestUIAddBasecoatWritesTheProofOfTheToaster(t *testing.T) {
	dir, fetch := uiProject(t)

	if code, _, errOut := runUI(t, dir, fetch, "ui", "add", "basecoat"); code != 0 {
		t.Fatalf("the command gave %d and said %q", code, errOut)
	}
	body := read(t, dir, "toaster_test.go")
	for _, want := range []string{
		`id="toaster"`,
		`data-category="success"`,
		"data-toast-cancel",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("toaster_test.go holds no %q:\n%s", want, body)
		}
	}
}

// TestUIAddBasecoatKeepsTheProofOfAPerson proves that a second run does not
// overwrite toaster_test.go. A person owns the file after the first run.
func TestUIAddBasecoatKeepsTheProofOfAPerson(t *testing.T) {
	dir, fetch := uiProject(t)

	if code, _, errOut := runUI(t, dir, fetch, "ui", "add", "basecoat"); code != 0 {
		t.Fatalf("the first run gave %d and said %q", code, errOut)
	}
	mark := "// a person wrote this line\n"
	writeFile(t, dir, "toaster_test.go", mark)
	if code, _, errOut := runUI(t, dir, fetch, "ui", "add", "basecoat"); code != 0 {
		t.Fatalf("the second run gave %d and said %q", code, errOut)
	}
	if got := read(t, dir, "toaster_test.go"); got != mark {
		t.Fatalf("the second run changed toaster_test.go to %q", got)
	}
}

func TestUIAddBasecoatRunsTwoTimesWithOneResult(t *testing.T) {
	dir, fetch := uiProject(t)

	if code, _, errOut := runUI(t, dir, fetch, "ui", "add", "basecoat"); code != 0 {
		t.Fatalf("the first run gave %d and said %q", code, errOut)
	}
	first := read(t, dir, "assets/css/app.css")
	if code, _, errOut := runUI(t, dir, fetch, "ui", "add", "basecoat"); code != 0 {
		t.Fatalf("the second run gave %d and said %q", code, errOut)
	}
	if second := read(t, dir, "assets/css/app.css"); second != first {
		t.Fatalf("the second run changed the stylesheet to %q", second)
	}
	if strings.Count(read(t, dir, "assets/js/app.js"), "all.min.js") != 1 {
		t.Fatal("the second run added the import again")
	}
}

// TestUIAddBasecoatKeepsTheComponentOfAPerson proves that a second run does
// not overwrite the toaster component. A person owns the file after the first
// run.
func TestUIAddBasecoatKeepsTheComponentOfAPerson(t *testing.T) {
	dir, fetch := uiProject(t)

	if code, _, errOut := runUI(t, dir, fetch, "ui", "add", "basecoat"); code != 0 {
		t.Fatalf("the first run gave %d and said %q", code, errOut)
	}
	mark := "// a person wrote this line\n"
	writeFile(t, dir, "internal/ui/toaster.templ", mark)
	if code, _, errOut := runUI(t, dir, fetch, "ui", "add", "basecoat"); code != 0 {
		t.Fatalf("the second run gave %d and said %q", code, errOut)
	}
	if got := read(t, dir, "internal/ui/toaster.templ"); got != mark {
		t.Fatalf("the second run changed the component to %q", got)
	}
}

// TestUIAddBasecoatIsIdempotentAcrossReformatting proves that a line that a
// person reformatted, with different spacing, does not gain a second copy.
func TestUIAddBasecoatIsIdempotentAcrossReformatting(t *testing.T) {
	dir, fetch := uiProject(t)
	writeFile(t, dir, "assets/css/app.css",
		"@import \"tailwindcss\";\n\n  @import \"../vendor/basecoat/basecoat-vega.css\";  \n")

	code, _, errOut := runUI(t, dir, fetch, "ui", "add", "basecoat")
	if code != 0 {
		t.Fatalf("the command gave %d and said %q", code, errOut)
	}
	css := read(t, dir, "assets/css/app.css")
	if strings.Count(css, "basecoat-vega.css") != 1 {
		t.Fatalf("the stylesheet holds the import more than one time: %q", css)
	}
}

// TestUIAddBasecoatFailsWithNoTailwindImport proves that the command states a
// fault, and no repair with no word, when the anchor line is absent.
func TestUIAddBasecoatFailsWithNoTailwindImport(t *testing.T) {
	dir, fetch := uiProject(t)
	writeFile(t, dir, "assets/css/app.css", "body {\n    margin: 0;\n}\n")

	code, _, errOut := runUI(t, dir, fetch, "ui", "add", "basecoat")
	if code != 1 {
		t.Fatalf("the command gave %d, want 1", code)
	}
	if !strings.Contains(errOut, "assets/css/app.css") || !strings.Contains(errOut, "tailwindcss") {
		t.Fatalf("the fault does not name the file and the missing line: %q", errOut)
	}
	if _, err := os.Stat(filepath.Join(dir, "assets", "vendor", "basecoat")); !os.IsNotExist(err) {
		t.Fatalf("the vendor tree exists: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "avero.lock")); !os.IsNotExist(err) {
		t.Fatalf("the lock exists: %v", err)
	}
}

// TestUIAddBasecoatFailsWithNoDatastarImport proves that the command states a
// fault, and fetches nothing, when the script holds no anchor line.
func TestUIAddBasecoatFailsWithNoDatastarImport(t *testing.T) {
	dir, fetch := uiProject(t)
	writeFile(t, dir, "assets/js/app.js", "console.log(\"no datastar here\");\n")

	code, _, errOut := runUI(t, dir, fetch, "ui", "add", "basecoat")
	if code != 1 {
		t.Fatalf("the command gave %d, want 1", code)
	}
	if !strings.Contains(errOut, "assets/js/app.js") || !strings.Contains(errOut, "datastar") {
		t.Fatalf("the fault does not name the file and the missing line: %q", errOut)
	}
	if _, err := os.Stat(filepath.Join(dir, "assets", "vendor", "basecoat")); !os.IsNotExist(err) {
		t.Fatalf("the vendor tree exists: %v", err)
	}
}

// TestUIAddBasecoatRefusesAStyleFlagWithNoValue proves that a --style flag
// with no value gives a fault that names the flag, not the flag itself as an
// unknown argument.
func TestUIAddBasecoatRefusesAStyleFlagWithNoValue(t *testing.T) {
	dir, fetch := uiProject(t)

	code, _, errOut := runUI(t, dir, fetch, "ui", "add", "basecoat", "--style")
	if code != 1 {
		t.Fatalf("the command gave %d, want 1", code)
	}
	if !strings.Contains(errOut, "--style") || !strings.Contains(errOut, "names no value") {
		t.Fatalf("the fault does not name the absent value: %q", errOut)
	}
}

func TestUIAddBasecoatTakesAStyle(t *testing.T) {
	dir, fetch := uiProject(t)

	if code, _, errOut := runUI(t, dir, fetch, "ui", "add", "basecoat", "--style", "nova"); code != 0 {
		t.Fatalf("the command gave %d and said %q", code, errOut)
	}
	if css := read(t, dir, "assets/css/app.css"); !strings.Contains(css, "basecoat-nova.css") {
		t.Fatalf("the stylesheet is %q", css)
	}
}

func TestUIAddBasecoatRefusesAnUnknownStyle(t *testing.T) {
	dir, fetch := uiProject(t)

	code, _, errOut := runUI(t, dir, fetch, "ui", "add", "basecoat", "--style", "orion")
	if code != 1 {
		t.Fatalf("the command gave %d, want 1", code)
	}
	if !strings.Contains(errOut, "vega") {
		t.Fatalf("the fault names no known style: %q", errOut)
	}
}

func TestUIAddRefusesAnUnknownLibrary(t *testing.T) {
	dir, fetch := uiProject(t)

	code, _, errOut := runUI(t, dir, fetch, "ui", "add", "bootstrap")
	if code != 1 {
		t.Fatalf("the command gave %d, want 1", code)
	}
	if !strings.Contains(errOut, "basecoat") {
		t.Fatalf("the fault names no known library: %q", errOut)
	}
}

func TestUIAddRefusesAShapeThatServesJSON(t *testing.T) {
	dir, fetch := uiProject(t)
	writeFile(t, dir, "avero.json", "{\"name\":\"orders\",\"shape\":\"api\"}\n")

	code, _, errOut := runUI(t, dir, fetch, "ui", "add", "basecoat")
	if code != 1 {
		t.Fatalf("the command gave %d, want 1", code)
	}
	if !strings.Contains(errOut, "ssr") {
		t.Fatalf("the fault states no repair: %q", errOut)
	}
}

// scaffoldLayout returns the exact body that
// scaffold/templates/ssr/internal/ui/layout.templ.tmpl writes for the
// application that uiProject names, so a test compares against one source and
// not against a second copy that can drift from the template.
func scaffoldLayout(t *testing.T) string {
	t.Helper()
	body, err := scaffold.Layout("blog")
	if err != nil {
		t.Fatalf("scaffold.Layout returned %v, want nil", err)
	}
	return string(body)
}

func TestUIAddBasecoatPatchesALayoutThatItRecognizes(t *testing.T) {
	dir, fetch := uiProject(t)
	writeFile(t, dir, "internal/ui/layout.templ", scaffoldLayout(t))

	if code, out, errOut := runUI(t, dir, fetch, "ui", "add", "basecoat"); code != 0 {
		t.Fatalf("the command gave %d, said %q and %q", code, out, errOut)
	}

	layout := read(t, dir, "internal/ui/layout.templ")
	if strings.Contains(layout, "templ Toasts()") {
		t.Fatal("the layout still defines Toasts, so the application holds two definitions")
	}
	call := strings.Index(layout, "@Toasts()")
	main := strings.Index(layout, "</main>")
	end := strings.Index(layout, "</body>")
	if call < 0 || end < 0 || call < main || call > end {
		t.Fatalf("the call of Toasts does not stand between main and the end of the body:\n%s", layout)
	}
}

// TestUIAddBasecoatIndentsTheMovedCall proves that the moved call carries the
// indentation of the other children of the body element, and not the
// indentation of a child of main.
func TestUIAddBasecoatIndentsTheMovedCall(t *testing.T) {
	dir, fetch := uiProject(t)
	writeFile(t, dir, "internal/ui/layout.templ", scaffoldLayout(t))

	if code, _, errOut := runUI(t, dir, fetch, "ui", "add", "basecoat"); code != 0 {
		t.Fatalf("the command gave %d and said %q", code, errOut)
	}

	layout := read(t, dir, "internal/ui/layout.templ")
	for _, line := range strings.Split(layout, "\n") {
		if strings.TrimSpace(line) != "@Toasts()" {
			continue
		}
		if !strings.HasPrefix(line, "\t\t\t@Toasts()") || strings.HasPrefix(line, "\t\t\t\t@Toasts()") {
			t.Fatalf("the call carries the wrong indentation: %q", line)
		}
		return
	}
	t.Fatalf("the layout carries no call of Toasts:\n%s", layout)
}

func TestUIAddBasecoatIsIdempotentWhenTheLayoutAlreadyCarriesThePatch(t *testing.T) {
	dir, fetch := uiProject(t)
	writeFile(t, dir, "internal/ui/layout.templ", scaffoldLayout(t))

	if code, _, errOut := runUI(t, dir, fetch, "ui", "add", "basecoat"); code != 0 {
		t.Fatalf("the first run gave %d and said %q", code, errOut)
	}
	patched := read(t, dir, "internal/ui/layout.templ")

	code, out, errOut := runUI(t, dir, fetch, "ui", "add", "basecoat")
	if code != 0 {
		t.Fatalf("the second run gave %d and said %q", code, errOut)
	}
	if got := read(t, dir, "internal/ui/layout.templ"); got != patched {
		t.Fatalf("the second run changed a layout that already carries the patch:\n%s", got)
	}
	if strings.Contains(out, "does not match the scaffold") {
		t.Fatalf("the second run asked for a change by hand: %q", out)
	}
}

func TestUIAddBasecoatLeavesAChangedLayoutAlone(t *testing.T) {
	dir, fetch := uiProject(t)
	changed := "package ui\n\ntempl Page(title string, body templ.Component) {\n\t<html><body>@body</body></html>\n}\n"
	writeFile(t, dir, "internal/ui/layout.templ", changed)

	code, out, errOut := runUI(t, dir, fetch, "ui", "add", "basecoat")
	if code != 0 {
		t.Fatalf("the command gave %d and said %q", code, errOut)
	}
	if read(t, dir, "internal/ui/layout.templ") != changed {
		t.Fatal("the command changed a layout that it does not recognize")
	}
	if !strings.Contains(out, "@Toasts()") {
		t.Fatalf("the command printed no line to paste: %q", out)
	}
}

// TestUIAddBasecoatFailsWhenTheLayoutDoesNotRead proves that a fault of
// os.ReadFile, beyond an absent file, gives a fault and not a silent skip.
func TestUIAddBasecoatFailsWhenTheLayoutDoesNotRead(t *testing.T) {
	dir, fetch := uiProject(t)
	layout := filepath.Join(dir, "internal", "ui", "layout.templ")
	if err := os.MkdirAll(layout, 0o755); err != nil {
		t.Fatalf("MkdirAll returned %v", err)
	}

	code, _, errOut := runUI(t, dir, fetch, "ui", "add", "basecoat")
	if code != 1 {
		t.Fatalf("the command gave %d, want 1", code)
	}
	if !strings.Contains(errOut, "internal/ui/layout.templ") {
		t.Fatalf("the fault does not name the file: %q", errOut)
	}
}

// TestUIAddBasecoatLeavesALayoutWithAnAddedElementAlone proves that a layout
// that carries an element beyond the scaffold, such as a footer that a person
// wrote, does not lose that work. The command compares the whole file, so a
// layout with an added element matches neither the scaffold nor the patch.
func TestUIAddBasecoatLeavesALayoutWithAnAddedElementAlone(t *testing.T) {
	dir, fetch := uiProject(t)
	changed := strings.Replace(scaffoldLayout(t), "</main>",
		"</main>\n\t\t\t<footer>Written by a person</footer>", 1)
	writeFile(t, dir, "internal/ui/layout.templ", changed)

	code, out, errOut := runUI(t, dir, fetch, "ui", "add", "basecoat")
	if code != 0 {
		t.Fatalf("the command gave %d and said %q", code, errOut)
	}
	if read(t, dir, "internal/ui/layout.templ") != changed {
		t.Fatal("the command changed a layout that carries an element beyond the scaffold")
	}
	if !strings.Contains(out, "does not match the scaffold") {
		t.Fatalf("the command printed no line to paste: %q", out)
	}
}
