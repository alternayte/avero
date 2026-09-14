package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// front writes a package.json and returns the directory of a front end.
func front(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(body), 0o644); err != nil {
		t.Fatalf("WriteFile returned %v", err)
	}
	return dir
}

// tool returns the package manager of one project and one front end.
func tool(t *testing.T, project *Project, dir string) NodeTool {
	t.Helper()
	out, err := NodeToolOf(project, dir)
	if err != nil {
		t.Fatalf("NodeToolOf returned %v", err)
	}
	return out
}

// avero.json names the tool of the application, so a front end of Bun never
// calls npm.
func TestTheProjectNamesThePackageManager(t *testing.T) {
	dir := front(t, `{"scripts":{"dev":"vite"}}`)
	project := &Project{}
	project.Assets.PackageManager = Bun
	got := tool(t, project, dir)
	if got.Name != Bun {
		t.Fatalf("the tool is %q", got.Name)
	}
	if strings.Join(got.Install(), " ") != "bun install" {
		t.Fatalf("the install command is %q", got.Install())
	}
	if strings.Join(got.Script(ScriptDev), " ") != "bun run dev" {
		t.Fatalf("the dev command is %q", got.Script(ScriptDev))
	}
}

// package.json states the tool in the member that Corepack names.
func TestThePackageFileNamesThePackageManager(t *testing.T) {
	dir := front(t, `{"packageManager":"pnpm@9.1.0"}`)
	if got := tool(t, &Project{}, dir); got.Name != PNPM {
		t.Fatalf("the tool is %q", got.Name)
	}
}

// A lock file names the tool that wrote it.
func TestTheLockFileNamesThePackageManager(t *testing.T) {
	dir := front(t, `{}`)
	if err := os.WriteFile(filepath.Join(dir, "bun.lock"), []byte("{}\n"), 0o644); err != nil {
		t.Fatalf("WriteFile returned %v", err)
	}
	if got := tool(t, &Project{}, dir); got.Name != Bun {
		t.Fatalf("the tool is %q", got.Name)
	}
}

// A front end that states nothing takes npm, because every machine that holds
// Node.js holds npm.
func TestThePackageManagerFallsBackToNPM(t *testing.T) {
	if got := tool(t, &Project{}, front(t, `{}`)); got.Name != NPM {
		t.Fatalf("the tool is %q", got.Name)
	}
}

// The application replaces the command of one script.
func TestTheProjectReplacesTheCommandOfAScript(t *testing.T) {
	project := &Project{}
	project.Assets.Dev = []string{"mise", "run", "front"}
	got := tool(t, project, front(t, `{}`))
	if strings.Join(got.Script(ScriptDev), " ") != "mise run front" {
		t.Fatalf("the dev command is %q", got.Script(ScriptDev))
	}
}

// A name that Avero does not drive states the repair.
func TestAnUnknownPackageManagerIsAFault(t *testing.T) {
	project := &Project{}
	project.Assets.PackageManager = "deno"
	_, err := NodeToolOf(project, front(t, `{}`))
	if err == nil || !strings.Contains(err.Error(), "packageManager") {
		t.Fatalf("the fault is %v", err)
	}
}

// A front end that states no script for the client of the API runs none.
func TestHasScriptReadsThePackageFile(t *testing.T) {
	got := tool(t, &Project{}, front(t, `{"scripts":{"api":"openapi-ts"}}`))
	if !got.HasScript(ScriptAPI) {
		t.Fatal("the front end states the api script")
	}
	if got.HasScript(ScriptDev) {
		t.Fatal("the front end states no dev script")
	}
}

// drel refuses an empty modules block, so Avero reads the block before it runs
// the generator.
func TestDrelModulesReadsTheBlock(t *testing.T) {
	for source, want := range map[string]bool{
		"modules: []\ndialect: sqlite\n":                           false,
		"modules:\ndialect: sqlite\n":                              false,
		"modules:\n  # a comment\ndialect: sqlite\n":               false,
		"modules:\n  - name: makers\n    packages:\n      - ./m\n": true,
		"dialect: sqlite\nmodules:\n  - name: makers\n":            true,
	} {
		if got := drelModules(source); got != want {
			t.Fatalf("drelModules(%q) is %v", source, got)
		}
	}
}
