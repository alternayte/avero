package cli_test

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alternayte/avero/internal/cli"
)

// run performs one command in a directory and returns the code and the output.
func run(t *testing.T, dir string, args ...string) (int, string, string) {
	t.Helper()
	var out, errOut bytes.Buffer
	code := cli.Run(context.Background(), cli.Streams{Out: &out, Err: &errOut, Dir: dir}, args)
	return code, out.String(), errOut.String()
}

func TestHelpListsEveryCommandAndTheOnesThatWait(t *testing.T) {
	code, out, _ := run(t, t.TempDir(), "help")
	if code != 0 {
		t.Fatalf("the code is %d, want 0", code)
	}
	for _, name := range []string{"new", "slice", "generate", "build", "migrate", "routes", "modules",
		"schema", "doctor", "verify", "js", "assets", "dev", "mcp", "version"} {
		if !strings.Contains(out, name) {
			t.Fatalf("the help holds no %q:\n%s", name, out)
		}
	}
	for _, name := range []string{"avero es replay", "avero outbox dead", "S7", "S9"} {
		if !strings.Contains(out, name) {
			t.Fatalf("the help does not name %q:\n%s", name, out)
		}
	}
}

func TestACommandThatWaitsNamesItsSubsystem(t *testing.T) {
	code, _, errOut := run(t, t.TempDir(), "es", "replay", "orders")
	if code != 1 {
		t.Fatalf("the code is %d, want 1", code)
	}
	if !strings.Contains(errOut, "S9") {
		t.Fatalf("the fault does not name the subsystem: %q", errOut)
	}
}

func TestAnUnknownCommandStatesTheRepair(t *testing.T) {
	code, _, errOut := run(t, t.TempDir(), "deploy")
	if code != 1 || !strings.Contains(errOut, "avero help") {
		t.Fatalf("the code is %d and the fault is %q", code, errOut)
	}
}

func TestNewWithNoNameStatesTheRepair(t *testing.T) {
	code, _, errOut := run(t, t.TempDir(), "new")
	if code != 1 || !strings.Contains(errOut, "avero new <name>") {
		t.Fatalf("the code is %d and the fault is %q", code, errOut)
	}
}

func TestNewWritesTheApplication(t *testing.T) {
	dir := t.TempDir()
	code, out, errOut := run(t, dir, "new", "blog", "--shape", "ssr")
	if code != 0 {
		t.Fatalf("the code is %d: %s", code, errOut)
	}
	if !strings.Contains(out, "The application blog is ready") {
		t.Fatalf("the output holds %q", out)
	}
	for _, name := range []string{"main.go", "avero.json", "internal/features/posts/zz_generated.go"} {
		if _, err := os.Stat(filepath.Join(dir, "blog", filepath.FromSlash(name))); err != nil {
			t.Fatalf("the command wrote no %s", name)
		}
	}
}

func TestNewRefusesAnUnknownFlag(t *testing.T) {
	code, _, errOut := run(t, t.TempDir(), "new", "blog", "--template", "x")
	if code != 1 || !strings.Contains(errOut, "avero help new") {
		t.Fatalf("the code is %d and the fault is %q", code, errOut)
	}
}

func TestVersionPrintsTheVersion(t *testing.T) {
	code, out, _ := run(t, t.TempDir(), "version")
	if code != 0 || !strings.HasPrefix(out, "avero ") {
		t.Fatalf("the code is %d and the output is %q", code, out)
	}
}

func TestGenerateWritesTheBindingOfAnInput(t *testing.T) {
	dir := t.TempDir()
	source := `package api

import "github.com/alternayte/avero/router"

type Module struct{}

type CreateInput struct {
	Title string ` + "`form:\"title\" validate:\"required\"`" + `
}

func (m *Module) Create(c *router.Ctx, in CreateInput) (router.Response, error) {
	return router.NoContent(), nil
}
`
	if err := os.WriteFile(filepath.Join(dir, "api.go"), []byte(source), 0o644); err != nil {
		t.Fatalf("WriteFile returned %v", err)
	}
	if code, _, errOut := run(t, dir, "generate"); code != 0 {
		t.Fatalf("the code is %d: %s", code, errOut)
	}
	if _, err := os.Stat(filepath.Join(dir, "zz_generated.go")); err != nil {
		t.Fatal("the command wrote no generated file")
	}
	if code, _, _ := run(t, dir, "generate", "--check"); code != 0 {
		t.Fatal("the check reports a file that the command just wrote")
	}
	if err := os.WriteFile(filepath.Join(dir, "zz_generated.go"), []byte("package api\n"), 0o644); err != nil {
		t.Fatalf("WriteFile returned %v", err)
	}
	code, _, errOut := run(t, dir, "generate", "--check")
	if code != 1 || !strings.Contains(errOut, "avero generate") {
		t.Fatalf("the code is %d and the fault is %q", code, errOut)
	}
}

func TestAssetsInitWritesThePackageFile(t *testing.T) {
	dir := t.TempDir()
	code, out, errOut := run(t, dir, "assets", "init")
	if code != 0 {
		t.Fatalf("the code is %d: %s", code, errOut)
	}
	if !strings.Contains(out, "package.json") {
		t.Fatalf("the output holds %q", out)
	}
	if _, err := os.Stat(filepath.Join(dir, "package.json")); err != nil {
		t.Fatal("the command wrote no package.json")
	}
}

func TestMigrateNewWritesTheFilePair(t *testing.T) {
	dir := t.TempDir()
	code, out, errOut := run(t, dir, "migrate", "new", "add posts")
	if code != 0 {
		t.Fatalf("the code is %d: %s", code, errOut)
	}
	lines := strings.Fields(strings.TrimSpace(out))
	if len(lines) != 2 {
		t.Fatalf("the command wrote %v", lines)
	}
	if !strings.HasSuffix(lines[0], "_add_posts.up.sql") || !strings.HasSuffix(lines[1], "_add_posts.down.sql") {
		t.Fatalf("the command wrote %v", lines)
	}
}

func TestMigrateUpNamesTheAbsentVariable(t *testing.T) {
	t.Setenv("DATABASE_URL", "")
	code, _, errOut := run(t, t.TempDir(), "migrate", "up")
	if code != 1 || !strings.Contains(errOut, "DATABASE_URL") {
		t.Fatalf("the code is %d and the fault is %q", code, errOut)
	}
}

func TestRoutesNamesTheRepairOutsideAModule(t *testing.T) {
	code, _, errOut := run(t, t.TempDir(), "routes")
	if code != 1 || !strings.Contains(errOut, "avero new") {
		t.Fatalf("the code is %d and the fault is %q", code, errOut)
	}
}

func TestAFlagThatIsNotKnownStatesTheRepair(t *testing.T) {
	code, _, errOut := run(t, t.TempDir(), "routes", "--yaml")
	if code != 1 || !strings.Contains(errOut, "--json") {
		t.Fatalf("the code is %d and the fault is %q", code, errOut)
	}
}

func TestDevRefusesAnEnvironmentThatIsNotDevelopment(t *testing.T) {
	t.Setenv("AVERO_ENV", "production")
	code, _, errOut := run(t, t.TempDir(), "dev")
	if code != 1 || !strings.Contains(errOut, "AVERO_ENV=development") {
		t.Fatalf("the code is %d and the fault is %q", code, errOut)
	}
}

func TestDevNamesTheAbsentSecret(t *testing.T) {
	t.Setenv("AVERO_ENV", "development")
	t.Setenv("AVERO_SECRET", "")
	code, _, errOut := run(t, t.TempDir(), "dev")
	if code != 1 || !strings.Contains(errOut, ".env.example") {
		t.Fatalf("the code is %d and the fault is %q", code, errOut)
	}
}

func TestDevRefusesAPortThatIsNotANumber(t *testing.T) {
	code, _, errOut := run(t, t.TempDir(), "dev", "--port", "eight")
	if code != 1 || !strings.Contains(errOut, "--port 8080") {
		t.Fatalf("the code is %d and the fault is %q", code, errOut)
	}
}

func TestTheEnvironmentFileReachesTheLoop(t *testing.T) {
	dir := t.TempDir()
	body := "# a comment\n\nexport AVERO_SECRET=\"abc\"\nPORT=9000\nbroken\n"
	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte(body), 0o644); err != nil {
		t.Fatalf("WriteFile returned %v", err)
	}
	t.Setenv("AVERO_ENV", "development")
	t.Setenv("AVERO_SECRET", "")
	// The loop reads .env, so the secret of the file is enough. The command
	// stops at the build, because the directory holds no application.
	code, _, errOut := run(t, dir, "dev")
	if code != 1 {
		t.Fatalf("the code is %d, want 1", code)
	}
	if strings.Contains(errOut, "AVERO_SECRET is absent") {
		t.Fatalf("the loop did not read .env: %q", errOut)
	}
}

func TestMCPServesOverTheStreamsOfTheCommand(t *testing.T) {
	// The command speaks the protocol on its own streams, so a test drives it
	// with no process.
	var out, errOut bytes.Buffer
	line := `{"jsonrpc":"2.0","id":1,"method":"tools/list"}` + "\n"
	code := cli.Run(context.Background(), cli.Streams{
		Out: &out, Err: &errOut, In: strings.NewReader(line), Dir: t.TempDir(),
	}, []string{"mcp"})
	if code != 0 {
		t.Fatalf("the code is %d: %s", code, errOut.String())
	}
	for _, tool := range []string{"list_modules", "scaffold_slice", "run_verify", "explain_error"} {
		if !strings.Contains(out.String(), tool) {
			t.Fatalf("the answer holds no %q:\n%s", tool, out.String())
		}
	}
}

func TestMCPRefusesAFlag(t *testing.T) {
	code, _, errOut := run(t, t.TempDir(), "mcp", "--stdio")
	if code != 1 || !strings.Contains(errOut, "avero mcp") {
		t.Fatalf("the code is %d and the fault is %q", code, errOut)
	}
}

func TestSliceWritesAFeature(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module blog\n\ngo 1.26.2\n"), 0o644); err != nil {
		t.Fatalf("WriteFile returned %v", err)
	}
	code, out, errOut := run(t, dir, "slice", "comment")
	if code != 0 {
		t.Fatalf("the code is %d: %s", code, errOut)
	}
	for _, name := range []string{"module.go", "handlers.go", "input.go", "store.go", "comments_test.go"} {
		if _, err := os.Stat(filepath.Join(dir, "internal", "features", "comments", name)); err != nil {
			t.Fatalf("the command wrote no %s", name)
		}
	}
	if !strings.Contains(out, "avero migrate up") {
		t.Fatalf("the output states no next step:\n%s", out)
	}
	// A second run refuses, because the slice already exists.
	if code, _, errOut := run(t, dir, "slice", "comment"); code != 1 || !strings.Contains(errOut, "already exists") {
		t.Fatalf("the code is %d and the fault is %q", code, errOut)
	}
}

func TestSliceNeedsAName(t *testing.T) {
	code, _, errOut := run(t, t.TempDir(), "slice")
	if code != 1 || !strings.Contains(errOut, "avero slice <name>") {
		t.Fatalf("the code is %d and the fault is %q", code, errOut)
	}
}
