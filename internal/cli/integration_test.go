//go:build integration

package cli_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/alternayte/avero/internal/budget"
	"github.com/alternayte/avero/internal/cli"
)

// The integration suite scaffolds each shape, builds it and runs its
// acceptance suite. It measures DX-1 and writes the result into artifacts.
// See the SDD, S14, and DX-1.

// repoRoot returns the directory of this repository.
func repoRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatalf("Abs returned %v", err)
	}
	return root
}

// goRun runs the Go tool in a directory.
func goRun(t *testing.T, dir string, args ...string) (string, error) {
	t.Helper()
	cmd := exec.Command("go", args...)
	cmd.Dir = dir
	cmd.Env = os.Environ()
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func TestEachShapeScaffoldsBuildsAndPassesItsSuite(t *testing.T) {
	root := repoRoot(t)
	for _, shape := range []string{"ssr", "spa", "api"} {
		t.Run(shape, func(t *testing.T) {
			dir := t.TempDir()
			code, _, errOut := run(t, dir, "new", shape+"app", "--shape", shape, "--replace", root)
			if code != 0 {
				t.Fatalf("avero new returned %d: %s", code, errOut)
			}
			app := filepath.Join(dir, shape+"app")
			if out, err := goRun(t, app, "mod", "tidy"); err != nil {
				t.Fatalf("go mod tidy failed: %v\n%s", err, out)
			}
			if out, err := goRun(t, app, "vet", "./..."); err != nil {
				t.Fatalf("go vet failed: %v\n%s", err, out)
			}
			if out, err := goRun(t, app, "test", "./...", "-count=1", "-race"); err != nil {
				t.Fatalf("the acceptance suite failed: %v\n%s", err, out)
			}
		})
	}
}

func TestTheDoctorReportsAnAbsentVariableAndAPendingMigration(t *testing.T) {
	root := repoRoot(t)
	dir := t.TempDir()
	if code, _, errOut := run(t, dir, "new", "blog", "--replace", root); code != 0 {
		t.Fatalf("avero new returned %d: %s", code, errOut)
	}
	app := filepath.Join(dir, "blog")
	if out, err := goRun(t, app, "mod", "tidy"); err != nil {
		t.Fatalf("go mod tidy failed: %v\n%s", err, out)
	}

	// No AVERO_SECRET, and the database holds no migration.
	t.Setenv("AVERO_SECRET", "")
	t.Setenv("DATABASE_URL", "file:"+filepath.Join(app, "doctor.db"))
	code, out, errOut := run(t, app, "doctor", "--json")
	if code != 1 {
		t.Fatalf("the code is %d, want 1:\n%s\n%s", code, out, errOut)
	}
	var report struct {
		Schema string `json:"schema"`
		OK     bool   `json:"ok"`
		Checks []struct {
			Name, State, Message, Repair string
		} `json:"checks"`
	}
	if err := json.Unmarshal([]byte(out), &report); err != nil {
		t.Fatalf("the report does not parse: %v\n%s", err, out)
	}
	if report.OK || report.Schema == "" {
		t.Fatalf("the report holds %+v", report)
	}
	found := map[string]string{}
	for _, row := range report.Checks {
		if row.State == "fail" {
			if row.Repair == "" {
				t.Fatalf("the row %q states no repair", row.Name)
			}
			found[row.Name] = row.Message
		}
	}
	if _, ok := found["the variable AVERO_SECRET"]; !ok {
		t.Fatalf("the doctor reports no absent variable: %+v", report.Checks)
	}
	if _, ok := found["the pending migrations"]; !ok {
		t.Fatalf("the doctor reports no pending migration: %+v", report.Checks)
	}
}

func TestTheRoutesAndTheModulesOfAScaffoldedApplication(t *testing.T) {
	root := repoRoot(t)
	dir := t.TempDir()
	if code, _, errOut := run(t, dir, "new", "blog", "--replace", root); code != 0 {
		t.Fatalf("avero new returned %d: %s", code, errOut)
	}
	app := filepath.Join(dir, "blog")
	if out, err := goRun(t, app, "mod", "tidy"); err != nil {
		t.Fatalf("go mod tidy failed: %v\n%s", err, out)
	}
	for _, tc := range []struct{ command, want string }{
		{"routes", "/posts/new"},
		{"modules", "posts"},
		{"schema", "posts"},
	} {
		code, out, errOut := run(t, app, tc.command, "--json")
		if code != 0 {
			t.Fatalf("avero %s returned %d: %s", tc.command, code, errOut)
		}
		if !strings.Contains(out, tc.want) {
			t.Fatalf("avero %s holds no %q:\n%s", tc.command, tc.want, out)
		}
		var doc map[string]any
		if err := json.Unmarshal([]byte(out), &doc); err != nil {
			t.Fatalf("the answer of avero %s does not parse: %v", tc.command, err)
		}
		if doc["schema"] == nil {
			t.Fatalf("the answer of avero %s carries no schema", tc.command)
		}
	}
}

func TestVerifyRunsTheGateAndValidatesAgainstItsSchema(t *testing.T) {
	root := repoRoot(t)
	dir := t.TempDir()
	if code, _, errOut := run(t, dir, "new", "blog", "--replace", root); code != 0 {
		t.Fatalf("avero new returned %d: %s", code, errOut)
	}
	app := filepath.Join(dir, "blog")
	if out, err := goRun(t, app, "mod", "tidy"); err != nil {
		t.Fatalf("go mod tidy failed: %v\n%s", err, out)
	}

	code, out, errOut := run(t, app, "verify", "--json")
	if code != 0 {
		t.Fatalf("avero verify returned %d:\n%s\n%s", code, out, errOut)
	}
	var report struct {
		Schema  string `json:"schema"`
		OK      bool   `json:"ok"`
		Records []struct {
			Step, State, Message, File string
			Line, Column               int
		} `json:"records"`
	}
	if err := json.Unmarshal([]byte(out), &report); err != nil {
		t.Fatalf("the report does not parse: %v\n%s", err, out)
	}
	if report.Schema != cli.VerifySchemaID || !report.OK {
		t.Fatalf("the report holds %+v", report)
	}
	steps := make([]string, 0, len(report.Records))
	for _, row := range report.Records {
		steps = append(steps, row.Step)
	}
	if strings.Join(steps, ",") != "gofmt,vet,generate,test,build" {
		t.Fatalf("the steps are %v, and the order must not change", steps)
	}

	// A fault names the file and the line of the person.
	broken := filepath.Join(app, "broken.go")
	source := "package main\n\nfunc broken() int { return \"x\" }\n"
	if err := os.WriteFile(broken, []byte(source), 0o644); err != nil {
		t.Fatalf("WriteFile returned %v", err)
	}
	code, out, _ = run(t, app, "verify", "--json")
	if code != 1 {
		t.Fatalf("avero verify returned %d for a broken package:\n%s", code, out)
	}
	if !strings.Contains(out, "broken.go") {
		t.Fatalf("the report names no file:\n%s", out)
	}
}

// TestDX1 measures the time from `avero new` to an application that answers.
// The budget is 60 seconds. See DX-1.
func TestDX1(t *testing.T) {
	root := repoRoot(t)
	dir := t.TempDir()

	start := time.Now()
	if code, _, errOut := run(t, dir, "new", "blog", "--replace", root); code != 0 {
		t.Fatalf("avero new returned %d: %s", code, errOut)
	}
	app := filepath.Join(dir, "blog")
	if out, err := goRun(t, app, "mod", "tidy"); err != nil {
		t.Fatalf("go mod tidy failed: %v\n%s", err, out)
	}
	if out, err := goRun(t, app, "build", "-o", "app", "."); err != nil {
		t.Fatalf("go build failed: %v\n%s", err, out)
	}

	port := freePort(t)
	cmd := exec.Command("./app")
	cmd.Dir = app
	cmd.Env = append(os.Environ(),
		"PORT="+port,
		"AVERO_SECRET="+strings.Repeat("k", 64),
		"DATABASE_URL=file:"+filepath.Join(app, "dx1.db"),
		"MIGRATE_ON_BOOT=true")
	var log strings.Builder
	cmd.Stdout = &log
	cmd.Stderr = &log
	if err := cmd.Start(); err != nil {
		t.Fatalf("the application does not start: %v", err)
	}
	defer func() { _ = cmd.Process.Kill() }()

	address := "http://127.0.0.1:" + port + "/healthz"
	deadline := time.Now().Add(60 * time.Second)
	for time.Now().Before(deadline) {
		res, err := http.Get(address)
		if err == nil {
			_ = res.Body.Close()
			if res.StatusCode == http.StatusOK {
				break
			}
		}
		time.Sleep(50 * time.Millisecond)
	}
	elapsed := time.Since(start)

	res, err := http.Get(address)
	if err != nil {
		t.Fatalf("the application does not answer: %v\n%s", err, log.String())
	}
	_ = res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("the health endpoint answered %d", res.StatusCode)
	}
	if elapsed > 60*time.Second {
		t.Fatalf("DX-1 took %s, and the budget is 60s", elapsed.Round(time.Millisecond))
	}
	if err := budget.Record(root, "DX-1", "`avero new` to a running application",
		elapsed, 60*time.Second); err != nil {
		t.Fatalf("the measurement does not record: %v", err)
	}
	t.Logf("DX-1: %s", elapsed.Round(time.Millisecond))
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

// ctx keeps the context import for a future step of the gate.
var _ = context.Background
