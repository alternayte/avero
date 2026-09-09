package assets_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/alternayte/avero/assets"
)

// write puts one file into the tree of a test.
func write(t *testing.T, dir, name, body string) string {
	t.Helper()
	full := filepath.Join(dir, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatalf("MkdirAll returned %v", err)
	}
	if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
		t.Fatalf("WriteFile returned %v", err)
	}
	return full
}

// project returns a directory that holds a small SSR application.
func project(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	write(t, dir, "assets/js/app.js", "import { hello } from \"./util.js\";\nhello();\n")
	write(t, dir, "assets/js/util.js", "export function hello() { document.title = \"hi\"; }\n")
	write(t, dir, "assets/css/app.css", "@import \"./base.css\";\nh1 { color: red; }\n")
	write(t, dir, "assets/css/base.css", "body { margin: 0; }\n")
	return dir
}

// config returns the tier 0 configuration of the project.
func config(dir string) assets.Config {
	return assets.Config{
		Dir:     dir,
		Entries: []string{"js/app.js", "css/app.css"},
	}
}

func TestTierZeroBundlesWithNoNodeModules(t *testing.T) {
	dir := project(t)
	m, err := assets.Build(context.Background(), config(dir))
	if err != nil {
		t.Fatalf("Build returned %v, want nil", err)
	}
	for _, name := range []string{"js/app.js", "css/app.css", "app.js", "app.css"} {
		if !m.Has(name) {
			t.Fatalf("the manifest holds no %q, it holds %v", name, m.Names())
		}
	}
	entry, _ := m.Entry("css/app.css")
	body, err := os.ReadFile(filepath.Join(dir, "assets/dist", entry.File))
	if err != nil {
		t.Fatalf("ReadFile returned %v", err)
	}
	if !strings.Contains(string(body), "margin") || !strings.Contains(string(body), "red") {
		t.Fatalf("the bundle holds no import:\n%s", body)
	}
	if _, err := os.Stat(filepath.Join(dir, "assets/dist", assets.ManifestName)); err != nil {
		t.Fatalf("the build wrote no manifest: %v", err)
	}
}

func TestAChangedCSSFileProducesADifferentHash(t *testing.T) {
	dir := project(t)
	first, err := assets.Build(context.Background(), config(dir))
	if err != nil {
		t.Fatalf("Build returned %v, want nil", err)
	}
	write(t, dir, "assets/css/base.css", "body { margin: 8px; }\n")
	second, err := assets.Build(context.Background(), config(dir))
	if err != nil {
		t.Fatalf("Build returned %v, want nil", err)
	}
	a, _ := first.Entry("css/app.css")
	b, _ := second.Entry("css/app.css")
	if a.Hash == b.Hash {
		t.Fatalf("the hash stayed %q after the change", a.Hash)
	}
	if a.File == b.File {
		t.Fatalf("the file name stayed %q after the change", a.File)
	}
}

func TestTierZeroRefusesABareSpecifier(t *testing.T) {
	dir := project(t)
	write(t, dir, "assets/js/app.js", "import x from \"lodash\";\nconsole.log(x);\n")
	_, err := assets.Build(context.Background(), config(dir))
	if err == nil {
		t.Fatal("Build returned nil, want a fault")
	}
	msg := err.Error()
	if !strings.Contains(msg, "lodash") {
		t.Fatalf("the fault does not name the package:\n%s", msg)
	}
	if !strings.Contains(msg, "avero js pin") || !strings.Contains(msg, "avero assets init") {
		t.Fatalf("the fault states no repair:\n%s", msg)
	}
	if !strings.Contains(msg, filepath.FromSlash("assets/js/app.js")) {
		t.Fatalf("the fault does not name the file of the person:\n%s", msg)
	}
}

func TestTierZeroResolvesAVendoredPackage(t *testing.T) {
	dir := project(t)
	write(t, dir, "assets/js/vendor/nanostores.js", "export const atom = () => 1;\n")
	write(t, dir, "assets/js/app.js", "import { atom } from \"nanostores\";\nconsole.log(atom());\n")
	m, err := assets.Build(context.Background(), config(dir))
	if err != nil {
		t.Fatalf("Build returned %v, want nil", err)
	}
	entry, _ := m.Entry("js/app.js")
	body, err := os.ReadFile(filepath.Join(dir, "assets/dist", entry.File))
	if err != nil {
		t.Fatalf("ReadFile returned %v", err)
	}
	if !strings.Contains(string(body), "atom") {
		t.Fatalf("the bundle holds no vendored code:\n%s", body)
	}
}

func TestASyntaxFaultNamesTheFileTheLineAndTheColumn(t *testing.T) {
	dir := project(t)
	write(t, dir, "assets/js/app.js", "const x = ;\n")
	_, err := assets.Build(context.Background(), config(dir))
	if err == nil {
		t.Fatal("Build returned nil, want a fault")
	}
	var faults *assets.Faults
	if !errorsAs(err, &faults) {
		t.Fatalf("the error is %T, want *assets.Faults", err)
	}
	f := faults.Faults[0]
	if !strings.Contains(f.File, filepath.FromSlash("assets/js/app.js")) || f.Line != 1 {
		t.Fatalf("the fault names %s:%d:%d", f.File, f.Line, f.Column)
	}
}

func TestTierOneResolvesNodeModules(t *testing.T) {
	dir := project(t)
	write(t, dir, "package.json", `{"name":"app","private":true}`)
	write(t, dir, "node_modules/tiny/package.json", `{"name":"tiny","main":"index.js"}`)
	write(t, dir, "node_modules/tiny/index.js", "export const tiny = 42;\n")
	write(t, dir, "assets/js/app.js", "import { tiny } from \"tiny\";\nconsole.log(tiny);\n")

	cfg := config(dir)
	cfg.Tier = assets.TierNode
	m, err := assets.Build(context.Background(), cfg)
	if err != nil {
		t.Fatalf("Build returned %v, want nil", err)
	}
	entry, _ := m.Entry("js/app.js")
	body, err := os.ReadFile(filepath.Join(dir, "assets/dist", entry.File))
	if err != nil {
		t.Fatalf("ReadFile returned %v", err)
	}
	if !strings.Contains(string(body), "42") {
		t.Fatalf("the bundle holds no package:\n%s", body)
	}
}

func TestTierExternalRunsTheCommand(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the test writes a shell script")
	}
	dir := project(t)
	m := assets.NewManifest("/assets/")
	m.Set(assets.Entry{Source: "app.css", File: "app.deadbeef0000.css", Hash: "deadbeef0000", Size: 6, Integrity: "sha256-x"})
	doc, err := json.Marshal(m)
	if err != nil {
		t.Fatalf("Marshal returned %v", err)
	}
	script := write(t, dir, "bundle.sh", "#!/bin/sh\nset -e\nmkdir -p assets/dist\ncat > assets/dist/manifest.json <<'JSON'\n"+string(doc)+"\nJSON\nprintf 'body{}' > assets/dist/app.deadbeef0000.css\n")
	if err := os.Chmod(script, 0o755); err != nil {
		t.Fatalf("Chmod returned %v", err)
	}

	cfg := config(dir)
	cfg.Tier = assets.TierExternal
	cfg.Command = []string{"sh", "bundle.sh"}
	built, err := assets.Build(context.Background(), cfg)
	if err != nil {
		t.Fatalf("Build returned %v, want nil", err)
	}
	if got := built.Asset("app.css"); got != "/assets/app.deadbeef0000.css" {
		t.Fatalf("Asset = %q", got)
	}
}

func TestTierExternalNamesTheRepairWhenTheCommandWritesNoManifest(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the test writes a shell script")
	}
	dir := project(t)
	cfg := config(dir)
	cfg.Tier = assets.TierExternal
	cfg.Command = []string{"sh", "-c", "true"}
	_, err := assets.Build(context.Background(), cfg)
	if err == nil {
		t.Fatal("Build returned nil, want a fault")
	}
	if !strings.Contains(err.Error(), assets.ManifestName) {
		t.Fatalf("the fault does not name the manifest:\n%v", err)
	}
}

func TestBuildNamesTheRepairForAnAbsentEntry(t *testing.T) {
	dir := t.TempDir()
	_, err := assets.Build(context.Background(), config(dir))
	if err == nil {
		t.Fatal("Build returned nil, want a fault")
	}
	if !strings.Contains(err.Error(), "js/app.js") {
		t.Fatalf("the fault does not name the entry:\n%v", err)
	}
}

func TestTheBuildTakesAnIIFEModule(t *testing.T) {
	dir := project(t)
	// The scripts of Basecoat are IIFE files. An application imports one for
	// its side effect, so the bundle must hold its body.
	write(t, dir, "assets/vendor/basecoat/js/all.min.js",
		"(() => { window.basecoat = { initAll: () => 1 }; })();\n")
	write(t, dir, "assets/js/app.js",
		"import \"../vendor/basecoat/js/all.min.js\";\ndocument.title = \"hi\";\n")

	if _, err := assets.Build(context.Background(), config(dir)); err != nil {
		t.Fatalf("Build returned %v, want nil", err)
	}
	body, err := os.ReadFile(filepath.Join(dir, "assets", "dist", "js", "app.js"))
	if err != nil {
		t.Fatalf("the bundle is absent: %v", err)
	}
	if !strings.Contains(string(body), "initAll") {
		t.Fatalf("the bundle holds no body of the module: %s", body)
	}
}
