//go:build integration

package cli_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/alternayte/avero/internal/budget"
	"github.com/alternayte/avero/verify"
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
		Schema  string          `json:"schema"`
		OK      bool            `json:"ok"`
		Records []verify.Record `json:"records"`
	}
	if err := json.Unmarshal([]byte(out), &report); err != nil {
		t.Fatalf("the report does not parse: %v\n%s", err, out)
	}
	if report.Schema != verify.SchemaID || !report.OK {
		t.Fatalf("the report holds %+v", report)
	}
	steps := make([]string, 0, len(report.Records))
	for _, row := range report.Records {
		steps = append(steps, row.Name)
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

// TestTheSpaBinaryCarriesItsFrontEnd proves the single binary: npm and Vite
// build the TypeScript front end into the output directory, the compiler
// embeds it, and the binary serves it and its own migrations from a directory
// that holds nothing else.
func TestTheSpaBinaryCarriesItsFrontEnd(t *testing.T) {
	if _, err := exec.LookPath("npm"); err != nil {
		t.Skip("npm is absent, and the spa shape needs it")
	}
	root := repoRoot(t)
	dir := t.TempDir()
	if code, _, errOut := run(t, dir, "new", "board", "--shape", "spa", "--replace", root); code != 0 {
		t.Fatalf("avero new returned %d: %s", code, errOut)
	}
	app := filepath.Join(dir, "board")

	// `avero build` reads the dependencies, checks the types and writes the
	// bundle into assets/dist.
	code, out, errOut := run(t, app, "build")
	if code != 0 {
		t.Fatalf("avero build returned %d:\n%s\n%s", code, out, errOut)
	}
	// The build writes the description of the API and the client of the front
	// end from it, so a change of a handler reaches the types of the front
	// end.
	description, err := os.ReadFile(filepath.Join(app, "openapi.json"))
	if err != nil {
		t.Fatalf("the build wrote no description: %v", err)
	}
	if !strings.Contains(string(description), `"#/components/schemas/TaskList"`) {
		t.Fatalf("the description holds no answer schema:\n%s", description)
	}
	types, err := os.ReadFile(filepath.Join(app, "web", "src", "client", "types.gen.ts"))
	if err != nil {
		t.Fatalf("the build wrote no client: %v", err)
	}
	for _, want := range []string{"export type Task", "done: boolean", "title: string"} {
		if !strings.Contains(string(types), want) {
			t.Fatalf("the generated client holds no %q:\n%s", want, types)
		}
	}
	if _, err := os.Stat(filepath.Join(app, "web", "src", "client", "@tanstack", "react-query.gen.ts")); err != nil {
		t.Fatalf("the build wrote no query options: %v", err)
	}

	index, err := os.ReadFile(filepath.Join(app, "assets", "dist", "index.html"))
	if err != nil {
		t.Fatalf("the build wrote no index document: %v", err)
	}
	if !strings.Contains(string(index), "/assets/index-") {
		t.Fatalf("the index document names no bundle:\n%s", index)
	}

	binary := filepath.Join(app, "bin", "board")
	if _, err := os.Stat(binary); err != nil {
		t.Fatalf("the build wrote no binary: %v", err)
	}
	empty := t.TempDir()

	port := freePort(t)
	cmd := exec.Command(binary)
	// The binary runs in a directory that holds no asset, no migration and
	// no source.
	cmd.Dir = empty
	cmd.Env = append(os.Environ(),
		"PORT="+port,
		"AVERO_SECRET="+strings.Repeat("k", 64),
		"DATABASE_URL=file:"+filepath.Join(empty, "board.db"),
		"MIGRATE_ON_BOOT=true")
	var log strings.Builder
	cmd.Stdout, cmd.Stderr = &log, &log
	if err := cmd.Start(); err != nil {
		t.Fatalf("the application does not start: %v", err)
	}
	defer func() { _ = cmd.Process.Kill() }()

	address := "http://127.0.0.1:" + port
	waitForHealth(t, address+"/healthz", log.String)

	// The front end answers the root path and every path that it owns.
	for _, path := range []string{"/", "/tasks/7"} {
		page := body(t, address+path)
		if !strings.Contains(page, `id="root"`) || !strings.Contains(page, "/assets/index-") {
			t.Fatalf("%s holds no document of the front end:\n%s", path, page)
		}
	}

	// The bundle answers from the binary.
	name := bundleOf(t, string(index))
	bundle := body(t, address+name)
	if len(bundle) < 100_000 {
		t.Fatalf("the bundle holds %d bytes, want React and TanStack Query", len(bundle))
	}

	// The migrations of the binary reached the database, so the API answers.
	created := post(t, address+"/api/tasks", `{"title":"from the binary"}`)
	if !strings.Contains(created, "from the binary") {
		t.Fatalf("the API answered %q", created)
	}
	if list := body(t, address+"/api/tasks"); !strings.Contains(list, "from the binary") {
		t.Fatalf("the list holds %q", list)
	}
}

// bundleOf returns the address of the script of the index document.
func bundleOf(t *testing.T, index string) string {
	t.Helper()
	mark := `src="`
	i := strings.Index(index, "/assets/index-")
	if i < 0 {
		t.Fatalf("the index document names no bundle:\n%s", index)
	}
	_ = mark
	rest := index[i:]
	end := strings.IndexAny(rest, `"'`)
	if end < 0 {
		t.Fatalf("the address does not close:\n%s", index)
	}
	return rest[:end]
}

// post writes one JSON body and returns the answer.
func post(t *testing.T, address, body string) string {
	t.Helper()
	res, err := http.Post(address, "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatalf("Post returned %v", err)
	}
	defer func() { _ = res.Body.Close() }()
	out, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatalf("ReadAll returned %v", err)
	}
	return string(out)
}

// waitForHealth blocks until the application answers.
func waitForHealth(t *testing.T, address string, log func() string) {
	t.Helper()
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		res, err := http.Get(address)
		if err == nil {
			_ = res.Body.Close()
			if res.StatusCode == http.StatusOK {
				return
			}
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("the application did not answer:\n%s", log())
}

// body returns the answer of one address.
func body(t *testing.T, address string) string {
	t.Helper()
	res, err := http.Get(address)
	if err != nil {
		t.Fatalf("Get returned %v", err)
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("%s answered %d", address, res.StatusCode)
	}
	out, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatalf("ReadAll returned %v", err)
	}
	return string(out)
}

// TestTheOpenAPIDescriptionOfAScaffoldedApplication proves that the command
// reads a real application and writes a description that a client reads.
func TestTheOpenAPIDescriptionOfAScaffoldedApplication(t *testing.T) {
	root := repoRoot(t)
	dir := t.TempDir()
	if code, _, errOut := run(t, dir, "new", "orders", "--shape", "api", "--replace", root); code != 0 {
		t.Fatalf("avero new returned %d: %s", code, errOut)
	}
	app := filepath.Join(dir, "orders")

	code, out, errOut := run(t, app, "routes", "--openapi", "--server", "https://api.example.com")
	if code != 0 {
		t.Fatalf("the code is %d: %s", code, errOut)
	}
	var doc struct {
		OpenAPI string `json:"openapi"`
		Info    struct {
			Title string `json:"title"`
		} `json:"info"`
		Paths map[string]map[string]struct {
			OperationID string `json:"operationId"`
			Summary     string `json:"summary"`
			Parameters  []struct {
				Name string `json:"name"`
				In   string `json:"in"`
			} `json:"parameters"`
			RequestBody *struct {
				Content map[string]struct {
					Schema struct {
						Properties map[string]any `json:"properties"`
						Required   []string       `json:"required"`
					} `json:"schema"`
				} `json:"content"`
			} `json:"requestBody"`
			Responses map[string]any `json:"responses"`
		} `json:"paths"`
	}
	if err := json.Unmarshal([]byte(out), &doc); err != nil {
		t.Fatalf("the description does not parse: %v\n%s", err, out)
	}
	if doc.OpenAPI != "3.1.0" || doc.Info.Title != "orders" {
		t.Fatalf("the description holds %+v", doc)
	}
	create, ok := doc.Paths["/posts"]["post"]
	if !ok {
		t.Fatalf("the description holds no POST /posts: %v", doc.Paths)
	}
	if create.OperationID != "posts.Create" || create.Summary == "" {
		t.Fatalf("the operation is %+v", create)
	}
	if create.RequestBody == nil {
		t.Fatal("the operation carries no body")
	}
	body := create.RequestBody.Content["application/json"].Schema
	if _, ok := body.Properties["title"]; !ok {
		t.Fatalf("the body holds %v", body.Properties)
	}
	if strings.Join(body.Required, ",") != "body,title" {
		t.Fatalf("the required fields are %v", body.Required)
	}
	show, ok := doc.Paths["/posts/{id}"]["get"]
	if !ok || len(show.Parameters) != 1 || show.Parameters[0].In != "path" {
		t.Fatalf("the operation of GET /posts/{id} is %+v", show)
	}
	for _, code := range []string{"200", "422", "500"} {
		if _, ok := create.Responses[code]; !ok {
			t.Fatalf("the operation states no %s: %v", code, create.Responses)
		}
	}
}
