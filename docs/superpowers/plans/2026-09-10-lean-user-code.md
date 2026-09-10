# Lean User Code Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Remove about 145 lines of ceremony from each Avero application and about 43 lines from each new feature slice, with no change to the design rules of the SDD.

**Architecture:** Four additions to the root `avero` package and to the generator take over code that every application repeats. `avero.Serve` runs the start sequence that section 5.3 of the SDD already fixes. `avero.Repo` and `avero.Save` read the transaction of the request. `avero.MountAssets` mounts the built assets. `avero generate` writes the model description that a person retypes today. The scaffold templates then shrink, and `just examples` writes the three reference applications again.

**Tech Stack:** Go 1.24, `github.com/alternayte/drel` v0.7.1, `go/ast` for the generator, `just` for the gate.

**Spec:** `docs/superpowers/specs/2026-09-10-lean-user-code-design.md`

## Global Constraints

- Module path is `github.com/alternayte/avero`. The binary is `avero`.
- Write every comment, every document and every commit message in ASD-STE100 Simplified Technical English. Use the active voice. Use one word for one meaning. Do not use a contraction.
- Never edit a file under `examples/`. The examples are generated. Change `scaffold/templates`, then run `just examples`.
- Never edit a `zz_generated.go` file by hand. Change the generator.
- Write the test before the code that it judges.
- No global mutable state. No facade. No package-level singleton.
- No runtime reflection on a request path.
- Dependencies are struct fields. The constructor takes them.
- Generated code is plain Go that a person can read, edit and delete.
- Every new export carries a doc comment.
- The gate is `just verify`. A task is complete when `go test ./... -race -count=1`, `gofmt -l .`, `go vet ./...` and `golangci-lint run` all pass.
- Run `gofmt -w` on every file that you write, before you commit.

---

## File Structure

**Create:**

- `repo.go` — `Repo` and `Save`, the two transaction helpers of a store.
- `repo_test.go` — the tests of `repo.go`.
- `mount.go` — `MountAssets`.
- `mount_test.go` — the tests of `mount.go`.
- `serve.go` — `Configurer`, `Service` and `Serve`.
- `serve_test.go` — the tests of `serve.go`.
- `codegen/model.go` — the scan of a `model` package and the emission of the `Models` method.
- `codegen/model_test.go` — the tests of `codegen/model.go`.

**Modify:**

- `config/base.go` — add the `Base` method to `BaseConfig`.
- `module/module.go` — add the `ModelModule` interface.
- `module/inspect.go` — merge the models into the description.
- `codegen/scan.go` — add the `Models` field to `pkg`.
- `codegen/codegen.go` — read the sibling `model` directory in `build`.
- `codegen/emit.go` — call `emitModels`.
- `avero.go` — add the `ModelModule` alias.
- `scaffold/templates/{ssr,api,spa}/main.go.tmpl` — use `avero.Serve`.
- `scaffold/templates/{ssr,spa}/wire.go.tmpl` — use `avero.MountAssets`.
- `scaffold/templates/{ssr,api,spa}/internal/features/*/store.go.tmpl` — use `avero.Repo` and `avero.Save`.
- `scaffold/templates/{ssr,api,spa}/internal/features/*/module.go.tmpl` — delete the `Describe` method.
- `scaffold/templates/slice/{store.go.tmpl,module.go.tmpl}` — the same two changes.
- `docs/getting-started.md`, `docs/configuration.md`, `docs/agents.md` — state the new form.
- `docs/internal/sdd.md` — state `Serve` and `ModelModule`.

---

### Task 1: The transaction helpers

Every store repeats an unexported `repo` method and an unexported `save`
method. Both read the transaction of the request from the context. This task
moves both into the root package.

**Files:**
- Create: `repo.go`
- Create: `repo_test.go`

**Interfaces:**
- Consumes: nothing from an earlier task.
- Produces:
  - `func Repo[T any](ctx context.Context, meta drel.ModelMeta[T]) (*drel.TxRepository[T], error)`
  - `func Save(ctx context.Context) error`

- [ ] **Step 1: Write the failing test**

Create `repo_test.go`:

```go
package avero_test

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alternayte/drel"

	avero "github.com/alternayte/avero"
)

// row is a model that no generator wrote. Repo needs the meta value only to
// build the repository, so a zero meta proves the lookup of the transaction.
type row struct{}

// A call outside a transaction names the repair, so a person reads what to
// register in wire.go.
func TestRepoWithoutATransaction(t *testing.T) {
	_, err := avero.Repo(context.Background(), drel.ModelMeta[row]{})
	if err == nil {
		t.Fatal("Repo returned no error outside a transaction")
	}
	if !strings.Contains(err.Error(), "avero.Transaction") {
		t.Fatalf("the error does not state the repair: %v", err)
	}
}

// A call inside a transaction returns the repository of that transaction.
func TestRepoInsideATransaction(t *testing.T) {
	engine, err := drel.NewEngine("file:" + filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("the engine does not open: %v", err)
	}
	defer engine.Close()

	err = engine.WithTx(context.Background(), func(ctx context.Context) error {
		repo, err := avero.Repo(ctx, drel.ModelMeta[row]{})
		if err != nil {
			return err
		}
		if repo == nil {
			t.Fatal("Repo returned no repository inside a transaction")
		}
		return avero.Save(ctx)
	})
	if err != nil {
		t.Fatalf("the transaction does not run: %v", err)
	}
}

// Save outside a transaction names the same repair as Repo.
func TestSaveWithoutATransaction(t *testing.T) {
	if err := avero.Save(context.Background()); err == nil {
		t.Fatal("Save returned no error outside a transaction")
	}
}
```

- [ ] **Step 2: Run the test and prove that it fails**

Run: `go test . -race -run 'TestRepo|TestSave' -v`
Expected: FAIL, because `avero.Repo` and `avero.Save` do not exist.

- [ ] **Step 3: Write the implementation**

Create `repo.go`:

```go
package avero

import (
	"context"
	"errors"
	"fmt"

	"github.com/alternayte/drel"
)

// errNoTx states the fault of a call that runs outside a transaction, and it
// states the repair. See DX-7.
var errNoTx = errors.New(
	"the call needs a transaction: register avero.Transaction in wire.go, or wrap the call in engine.WithTx")

// Repo returns the repository of the transaction of the request.
//
// The transaction middleware opens the transaction, so a handler always holds
// one. A job that reads outside a request opens its own with engine.WithTx.
// A read therefore reads its own writes, and a write stages the change on the
// transaction. See the SDD, S4.
//
//	repo, err := avero.Repo(ctx, model.PostMeta)
func Repo[T any](ctx context.Context, meta drel.ModelMeta[T]) (*drel.TxRepository[T], error) {
	tx, ok := drel.FromContext(ctx)
	if !ok {
		return nil, errNoTx
	}
	return drel.NewTxRepository(tx, meta), nil
}

// Save writes the staged changes inside the transaction of the request.
//
// The transaction still commits at the end of the request. The flush only
// makes a later read of the same request see the write.
func Save(ctx context.Context) error {
	tx, ok := drel.FromContext(ctx)
	if !ok {
		return errNoTx
	}
	if err := tx.SaveChanges(ctx); err != nil {
		return fmt.Errorf("the change does not write: %w", err)
	}
	return nil
}
```

- [ ] **Step 4: Run the test and prove that it passes**

Run: `go test . -race -run 'TestRepo|TestSave' -v`
Expected: PASS

- [ ] **Step 5: Run the gate of the package**

Run: `gofmt -l . && go vet ./... && golangci-lint run && go test ./... -race -count=1`
Expected: no output from `gofmt`, and PASS from the tests.

- [ ] **Step 6: Commit**

```bash
git add repo.go repo_test.go
git commit -m "Add Repo and Save, so a store states no transaction lookup

Claude-Session: https://claude.ai/code/session_018m68TtjNW98aezz2u79Qiw"
```

---

### Task 2: The asset mount

`LoadManifest`, `fs.Sub` and `Mount` always appear together in `wire.go`, and
each one can fail. One call does the three.

**Files:**
- Create: `mount.go`
- Create: `mount_test.go`

**Interfaces:**
- Consumes: `Manifest`, `LoadManifest` and `AssetHandler`, which `avero.go` already exports.
- Produces: `func MountAssets(r *Router, dist fs.FS, root string) (*Manifest, error)`

- [ ] **Step 1: Write the failing test**

Create `mount_test.go`:

```go
package avero_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"

	avero "github.com/alternayte/avero"
)

// MountAssets reads the manifest and serves the built file at /assets/.
func TestMountAssets(t *testing.T) {
	dist := fstest.MapFS{
		"assets/dist/manifest.json": &fstest.MapFile{
			Data: []byte(`{"css/app.css":"css/app-abc123.css"}`),
		},
		"assets/dist/css/app-abc123.css": &fstest.MapFile{Data: []byte("body{}")},
	}

	r := avero.NewRouter()
	manifest, err := avero.MountAssets(r, dist, "assets/dist")
	if err != nil {
		t.Fatalf("MountAssets failed: %v", err)
	}
	if got := manifest.Path("css/app.css"); got != "css/app-abc123.css" {
		t.Fatalf("the manifest holds %q", got)
	}

	handler, err := r.Handler()
	if err != nil {
		t.Fatalf("the handler does not build: %v", err)
	}
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/assets/css/app-abc123.css", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("the asset answers %d", rec.Code)
	}
}

// An absent manifest names the fault and returns no manifest.
func TestMountAssetsWithNoManifest(t *testing.T) {
	if _, err := avero.MountAssets(avero.NewRouter(), fstest.MapFS{}, "assets/dist"); err == nil {
		t.Fatal("MountAssets returned no error for an absent manifest")
	}
}
```

**Note on `manifest.Path`:** the accessor of `assets.Manifest` may carry
another name. Run `grep -n "^func (m \*Manifest)" assets/manifest.go` and use
the accessor that the file states. Change the assertion of the test to match.

- [ ] **Step 2: Run the test and prove that it fails**

Run: `go test . -race -run TestMountAssets -v`
Expected: FAIL, because `avero.MountAssets` does not exist.

- [ ] **Step 3: Write the implementation**

Create `mount.go`:

```go
package avero

import (
	"io/fs"
	"path"

	"github.com/alternayte/avero/assets"
)

// MountAssets reads the manifest of the built assets and serves them at
// /assets/.
//
// root is the directory of the built output inside dist, such as
// "assets/dist". The manifest stands beside the files, so the caller states
// one path and not two. Mount strips the prefix, so the handler reads the
// path of the asset.
//
//	manifest, err := avero.MountAssets(r, dist, "assets/dist")
func MountAssets(r *Router, dist fs.FS, root string) (*Manifest, error) {
	manifest, err := assets.LoadManifest(dist, path.Join(root, "manifest.json"))
	if err != nil {
		return nil, err
	}
	files, err := fs.Sub(dist, root)
	if err != nil {
		return nil, err
	}
	r.Mount("/assets/", assets.Handler(files, manifest))
	return manifest, nil
}
```

- [ ] **Step 4: Run the test and prove that it passes**

Run: `go test . -race -run TestMountAssets -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
gofmt -w mount.go mount_test.go
git add mount.go mount_test.go
git commit -m "Add MountAssets, so wire.go states one call and not three

Claude-Session: https://claude.ai/code/session_018m68TtjNW98aezz2u79Qiw"
```

---

### Task 3: `Base` on the base configuration

`Serve` reads the port, the secret and the shutdown grace of an application.
The configuration of an application embeds `BaseConfig`, so one method on
`BaseConfig` gives `Serve` the access, and the application writes no code.

**Files:**
- Modify: `config/base.go`
- Modify: `config/base_test.go`, or create it if it is absent.

**Interfaces:**
- Consumes: nothing from an earlier task.
- Produces: `func (c BaseConfig) Base() BaseConfig`

- [ ] **Step 1: Write the failing test**

Append to `config/base_test.go`, in package `config_test`:

```go
// An application configuration embeds BaseConfig, so it carries Base and it
// satisfies the constraint of avero.Serve with no hand-written method.
func TestBaseReturnsTheEmbeddedConfiguration(t *testing.T) {
	type appConfig struct {
		config.BaseConfig
		DatabaseURL string
	}
	cfg := appConfig{BaseConfig: config.BaseConfig{Port: 9090}}
	if got := cfg.Base().Port; got != 9090 {
		t.Fatalf("Base returned the port %d", got)
	}
}
```

If `config/base_test.go` does not exist, create it with the package clause
`package config_test` and the imports `testing` and
`github.com/alternayte/avero/config`.

- [ ] **Step 2: Run the test and prove that it fails**

Run: `go test ./config -race -run TestBaseReturns -v`
Expected: FAIL with "cfg.Base undefined".

- [ ] **Step 3: Write the implementation**

Append to `config/base.go`, after the `BaseConfig` type:

```go
// Base returns the base configuration.
//
// The configuration of an application embeds BaseConfig, so it carries this
// method and avero.Serve reads the port, the secret and the shutdown grace of
// any application. An application writes no method of its own.
func (c BaseConfig) Base() BaseConfig { return c }
```

- [ ] **Step 4: Run the test and prove that it passes**

Run: `go test ./config -race -run TestBaseReturns -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
gofmt -w config/base.go config/base_test.go
git add config/base.go config/base_test.go
git commit -m "Add Base to BaseConfig, so Serve reads the configuration of any application

Claude-Session: https://claude.ai/code/session_018m68TtjNW98aezz2u79Qiw"
```

---

### Task 4: `avero.Serve`

`main.go` is identical in the three reference applications. This task moves
its whole body into the root package.

**Files:**
- Create: `serve.go`
- Create: `serve_test.go`

**Interfaces:**
- Consumes: `Configurer` needs `BaseConfig.Base` from Task 3.
- Produces:
  - `type Configurer interface { Base() BaseConfig }`
  - `type Service[C Configurer] struct { Args []string; Wire func(*drel.Engine, C) (*Router, *ModuleSet, error); DSN func(C) string; DSNEnv string; Migrations func() []fs.FS; Options []Option; Ctx context.Context; Out, Err io.Writer }`
  - `func Serve[C Configurer](s Service[C]) int`

- [ ] **Step 1: Write the failing test**

Create `serve_test.go`:

```go
package avero_test

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/alternayte/drel"

	avero "github.com/alternayte/avero"
)

// serveConfig is the configuration of the application under test. It embeds
// BaseConfig, so it carries Base and it needs no method of its own.
type serveConfig struct {
	avero.BaseConfig
}

// serveWire builds a router with one route and no module.
func serveWire(_ *drel.Engine, _ serveConfig) (*avero.Router, *avero.ModuleSet, error) {
	r := avero.NewRouter()
	r.Get("/{$}", func(c *avero.Ctx) (avero.Response, error) {
		return avero.Text(http.StatusOK, "ready"), nil
	})
	return r, avero.Modules(), nil
}

// An inspection command opens no database and reads no configuration, so
// `avero routes` runs on a machine with no database. See DX-8.
func TestServeInspects(t *testing.T) {
	var out strings.Builder
	code := avero.Serve(avero.Service[serveConfig]{
		Args: []string{"routes"},
		Wire: serveWire,
		Out:  &out,
		Err:  io.Discard,
	})
	if code != 0 {
		t.Fatalf("the inspection returned the code %d", code)
	}
	if !strings.Contains(out.String(), "/") {
		t.Fatalf("the route table is empty: %q", out.String())
	}
}

// A configuration fault stops the process with the code 1 before it serves.
func TestServeStopsOnAConfigurationFault(t *testing.T) {
	t.Setenv("AVERO_SECRET", "")
	var errOut strings.Builder
	code := avero.Serve(avero.Service[serveConfig]{
		Args: nil,
		Wire: serveWire,
		Out:  io.Discard,
		Err:  &errOut,
	})
	if code != 1 {
		t.Fatalf("the run returned the code %d", code)
	}
	if !strings.Contains(errOut.String(), "AVERO_SECRET") {
		t.Fatalf("the fault does not name the variable: %q", errOut.String())
	}
}

// Serve starts the application, serves the handler and stops on the end of
// the context.
func TestServeRunsAndStops(t *testing.T) {
	t.Setenv("AVERO_SECRET", strings.Repeat("a", 64))
	t.Setenv("PORT", "0")

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan int, 1)
	go func() {
		done <- avero.Serve(avero.Service[serveConfig]{
			Args:    nil,
			Wire:    serveWire,
			Ctx:     ctx,
			Out:     io.Discard,
			Err:     io.Discard,
			Options: []avero.Option{avero.WithoutSignals()},
		})
	}()

	time.Sleep(200 * time.Millisecond)
	cancel()

	select {
	case code := <-done:
		if code != 0 {
			t.Fatalf("the run returned the code %d", code)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("the run did not stop")
	}
}
```

**Note:** `TestServeRunsAndStops` binds a port. Read `host/host.go` and
`avero_test.go` first. If the existing tests pass a listener with
`avero.WithListener`, pass one here in the same way and delete the `PORT`
variable.

- [ ] **Step 2: Run the test and prove that it fails**

Run: `go test . -race -run TestServe -v`
Expected: FAIL, because `avero.Serve` and `avero.Service` do not exist.

- [ ] **Step 3: Write the implementation**

Create `serve.go`:

```go
package avero

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"os"

	"github.com/alternayte/drel"
)

// Configurer gives Serve the base configuration of an application.
//
// BaseConfig carries the method, so a configuration that embeds it satisfies
// the constraint and writes no code.
type Configurer interface {
	Base() BaseConfig
}

// Service states the parts of one application that Serve needs.
//
// The zero value of every optional field states a sensible default, so a
// simple application fills three fields. See the SDD, 5.3, for the sequence
// that Serve runs.
type Service[C Configurer] struct {
	// Args holds the command line without the name of the program. Pass
	// os.Args[1:].
	Args []string
	// Wire builds the router and the module set. An inspection command
	// passes a nil engine and a zero configuration.
	Wire func(engine *drel.Engine, cfg C) (*Router, *ModuleSet, error)
	// DSN reads the database address from the configuration. A nil value
	// starts an application with no database.
	DSN func(cfg C) string
	// DSNEnv names the variable that `avero doctor` reads for the database
	// address. An empty value reads DATABASE_URL.
	DSNEnv string
	// Migrations holds the migration files of each feature slice. drel
	// merges the sets in version order. A nil value applies no migration.
	Migrations func() []fs.FS
	// Options pass to New after the options that Serve builds, so an
	// application can add a component or a check of its own.
	Options []Option
	// Ctx is the context of the run. A nil value uses context.Background.
	Ctx context.Context
	// Out and Err receive the output. A nil value uses os.Stdout and
	// os.Stderr.
	Out, Err io.Writer
}

// Serve runs one Avero application and returns the exit code of the process.
//
// It runs the sequence of section 5.3 of the SDD: it answers an inspection
// command, it loads the configuration, it opens the database, it builds the
// router, it registers the boot checks and the migrator, and it runs the
// application until a signal or the end of the context.
//
// An application with an unusual start calls Load, New and Run itself. Serve
// adds no behaviour that those three do not hold.
//
//	func main() {
//	    os.Exit(avero.Serve(avero.Service[Config]{
//	        Args:       os.Args[1:],
//	        Wire:       wire,
//	        DSN:        func(c Config) string { return c.DatabaseURL },
//	        Migrations: migrationSets,
//	    }))
//	}
func Serve[C Configurer](s Service[C]) int {
	out, errOut := s.Out, s.Err
	if out == nil {
		out = os.Stdout
	}
	if errOut == nil {
		errOut = os.Stderr
	}
	ctx := s.Ctx
	if ctx == nil {
		ctx = context.Background()
	}

	// An inspection command reads the routes and the modules. It opens no
	// database and it reads no configuration, so `avero routes` works on a
	// machine with no database.
	if Inspecting(s.Args) {
		var zero C
		r, modules, err := s.Wire(nil, zero)
		if err != nil {
			return Exit(errOut, err)
		}
		return Inspect[C](s.Args, out, errOut, r, modules, s.doctorChecks()...)
	}

	cfg, err := Load[C](ctx)
	if err != nil {
		return Exit(errOut, err)
	}
	base := (*cfg).Base()

	var engine *drel.Engine
	if s.DSN != nil {
		engine, err = drel.NewEngine(s.DSN(*cfg))
		if err != nil {
			fmt.Fprintf(errOut,
				"the database does not open: %v\n  → Prove %s, and start the database\n",
				err, s.dsnEnv())
			return 1
		}
		defer engine.Close()
	}

	r, modules, err := s.Wire(engine, *cfg)
	if err != nil {
		return Exit(errOut, err)
	}
	handler, err := r.Handler()
	if err != nil {
		return Exit(errOut, err)
	}
	_ = modules

	// The boot checks read the engine that the application already opened,
	// so a start opens one connection pool and no more. See DX-8.
	checks := []Check{SecretCheck(base.Secret)}
	if engine != nil {
		checks = append(checks, DatabaseCheckOn(engine))
		if !base.MigrateOnBoot && s.Migrations != nil {
			checks = append(checks, MigrationCheckOnFS(engine, s.Migrations()...))
		}
	}

	opts := []Option{WithHandler(handler), WithChecks(checks...)}
	if engine != nil && s.Migrations != nil {
		opts = append(opts, WithMigrator(fsMigrator{engine: engine, sets: s.Migrations}))
	}
	opts = append(opts, s.Options...)

	return Exit(errOut, New(base, opts...).Run(ctx))
}

// dsnEnv returns the name of the variable that holds the database address.
func (s Service[C]) dsnEnv() string {
	if s.DSNEnv == "" {
		return "DATABASE_URL"
	}
	return s.DSNEnv
}

// doctorChecks returns the boot checks of `avero doctor`.
//
// The doctor holds no engine, so a check opens its own connection and closes
// it. The boot uses the engine of the application instead. See DX-8.
func (s Service[C]) doctorChecks() []Check {
	dsn := os.Getenv(s.dsnEnv())
	if dsn == "" || s.DSN == nil {
		return nil
	}
	checks := []Check{DatabaseCheck(dsn)}
	if s.Migrations != nil {
		checks = append(checks, MigrationCheckFS(dsn, s.Migrations()...))
	}
	return checks
}

// fsMigrator applies the pending migrations when MIGRATE_ON_BOOT is true.
//
// Each feature slice embeds its own migrations, and drel merges the sets in
// version order. The files come from the binary, so one artifact carries the
// server and the schema.
type fsMigrator struct {
	engine *drel.Engine
	sets   func() []fs.FS
}

// Migrate applies every migration that the database does not hold.
func (m fsMigrator) Migrate(ctx context.Context) error {
	_, err := m.engine.ApplyMigrationsFS(ctx, m.sets()...)
	return err
}
```

- [ ] **Step 4: Run the test and prove that it passes**

Run: `go test . -race -run TestServe -v`
Expected: PASS

- [ ] **Step 5: Run the whole unit gate**

Run: `gofmt -l . && go vet ./... && golangci-lint run && go test ./... -race -count=1`
Expected: no output from `gofmt`, and PASS from the tests.

- [ ] **Step 6: Commit**

```bash
git add serve.go serve_test.go
git commit -m "Add Serve, so main.go states the application and not the sequence

Claude-Session: https://claude.ai/code/session_018m68TtjNW98aezz2u79Qiw"
```

---

### Task 5: The model description of a module

The `Describe` method restates the model struct by hand, and no gate finds a
copy that drifted. This task adds the optional interface that the generator
fills in Task 6, and it merges the result into the description.

**Files:**
- Modify: `module/module.go`
- Modify: `module/inspect.go:140-190`
- Modify: `module/inspect_test.go`, or create it if it is absent.
- Modify: `avero.go:440-470`

**Interfaces:**
- Consumes: `Description`, `ModelDesc` and `FieldDesc` from `module/describe.go`.
- Produces:
  - `type ModelModule interface { Module; Models() []ModelDesc }`
  - `avero.ModelModule`, the alias of the same type.
  - `describe(row Contribution, types []string, models []ModelDesc) *Description`

- [ ] **Step 1: Write the failing test**

Append to `module/inspect_test.go`, in package `module_test`:

```go
// modelOnly implements ModelModule and no DescribeModule, so the module
// system builds the whole description. See AN-3.
type modelOnly struct{}

func (modelOnly) Name() string { return "posts" }

func (modelOnly) Models() []module.ModelDesc {
	return []module.ModelDesc{{
		Name:   "Post",
		Table:  "posts",
		Fields: []module.FieldDesc{{Name: "Title", Type: "string"}},
	}}
}

// The description of a module carries the models that the generator wrote.
func TestTheDescriptionCarriesTheModels(t *testing.T) {
	set := module.Modules(modelOnly{})
	if err := set.Err(); err != nil {
		t.Fatalf("the inspection failed: %v", err)
	}
	rows := set.Contributions()
	if len(rows) != 1 {
		t.Fatalf("the set holds %d rows", len(rows))
	}
	models := rows[0].Description.Models
	if len(models) != 1 || models[0].Table != "posts" {
		t.Fatalf("the description holds the models %+v", models)
	}
	if !slices.Contains(rows[0].Interfaces, "ModelModule") {
		t.Fatalf("the row names the interfaces %v", rows[0].Interfaces)
	}
}
```

**Note:** read `module/inspect.go` for the accessor that returns the rows. If
it does not carry the name `Contributions`, use the name that the file states.
Add the imports `slices`, `testing` and
`github.com/alternayte/avero/module`.

- [ ] **Step 2: Run the test and prove that it fails**

Run: `go test ./module -race -run TestTheDescriptionCarriesTheModels -v`
Expected: FAIL, because `ModelModule` does not exist and the description holds no model.

- [ ] **Step 3: Add the interface**

Append to `module/module.go`, after `SummaryModule`:

```go
// ModelModule states the persistent models of one module.
//
// `avero generate` reads the model package of the feature slice and writes
// the method, so a person states each field one time. The module system
// merges the result into the description that an agent reads. See AN-3.
type ModelModule interface {
	Module
	// Models returns the persistent models of the module.
	Models() []ModelDesc
}
```

- [ ] **Step 4: Merge the models into the description**

In `module/inspect.go`, in the `inspect` method, add this block before the
`DescribeModule` block:

```go
	var models []ModelDesc
	if h, ok := m.(ModelModule); ok {
		row.Interfaces = append(row.Interfaces, "ModelModule")
		models = h.Models()
	}
```

Change the `DescribeModule` block and the branch that follows it to this:

```go
	if h, ok := m.(DescribeModule); ok {
		row.Interfaces = append(row.Interfaces, "DescribeModule")
		d := h.Describe()
		if d.Name == "" {
			d.Name = name
		}
		if d.Routes == nil {
			d.Routes = row.Routes
		}
		// A hand-written description that states no model takes the models
		// of the generator, so the two never disagree.
		if d.Models == nil {
			d.Models = models
		}
		row.Description = &d
	} else {
		row.Description = describe(row, types, models)
	}
```

Change the `describe` function to this:

```go
// describe builds the description of a module that implements no
// DescribeModule. It states what the inspection found.
func describe(row Contribution, types []string, models []ModelDesc) *Description {
	return &Description{
		Name:        row.Module,
		Routes:      row.Routes,
		Models:      models,
		Projections: row.Projections,
		InboxTypes:  types,
	}
}
```

- [ ] **Step 5: Add the alias**

In `avero.go`, in the block of module type aliases, add this member after
`DescribeModule`:

```go
	// ModelModule states the persistent models of a module. `avero generate`
	// writes the method.
	ModelModule = module.ModelModule
```

- [ ] **Step 6: Run the test and prove that it passes**

Run: `go test ./module . -race -run 'TestTheDescriptionCarriesTheModels' -v`
Expected: PASS

- [ ] **Step 7: Run the whole unit gate**

Run: `gofmt -l . && go vet ./... && golangci-lint run && go test ./... -race -count=1`
Expected: no output from `gofmt`, and PASS from the tests.

- [ ] **Step 8: Commit**

```bash
git add module/module.go module/inspect.go module/inspect_test.go avero.go
git commit -m "Add ModelModule, so the description of a module states its models

Claude-Session: https://claude.ai/code/session_018m68TtjNW98aezz2u79Qiw"
```

---

### Task 6: The generator writes `Models`

The generator already writes a `Summaries` method on the module receiver from
the comment of each handler. This task writes a `Models` method on the same
receiver from the sibling `model` package.

**Files:**
- Create: `codegen/model.go`
- Create: `codegen/model_test.go`
- Modify: `codegen/scan.go:55-66`
- Modify: `codegen/codegen.go:86-120`
- Modify: `codegen/emit.go:18-30`

**Interfaces:**
- Consumes: `module.ModelDesc` and `module.FieldDesc` from Task 5.
- Produces:
  - `type model struct { Name, Table string; Fields []modelField }`
  - `type modelField struct { Name, Type string }`
  - `func modelsOf(fset *token.FileSet, dir string) ([]model, error)`
  - `func scanModels(fset *token.FileSet, files []*ast.File) []model`
  - `func emitModels(b *strings.Builder, p pkg, imports map[string]bool)`
  - `pkg.Models []model`

- [ ] **Step 1: Write the failing test**

Create `codegen/model_test.go`:

```go
package codegen

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The generator reads the model package of a feature slice and writes the
// Models method of its module. See AN-3.
func TestTheGeneratorWritesTheModels(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, "handlers.go"), `package posts

import "github.com/alternayte/avero/router"

type Module struct{}

// Show renders one post.
func (m *Module) Show(c *router.Ctx, in ShowInput) (router.Response, error) {
	return nil, nil
}
`)
	write(t, filepath.Join(dir, "input.go"), `package posts

type ShowInput struct {
	ID string `+"`path:\"id\"`"+`
}
`)
	modelDir := filepath.Join(dir, "model")
	if err := os.MkdirAll(modelDir, 0o750); err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(modelDir, "post.go"), `package model

import (
	"github.com/alternayte/drel"
	"github.com/google/uuid"
)

type Post struct {
	drel.Model[uuid.UUID]

	Title string `+"`db:\"title\"`"+`
	Body  string `+"`db:\"body\"`"+`
}
`)
	write(t, filepath.Join(modelDir, "post_drel.go"), `package model

import "github.com/alternayte/drel"

var PostMeta = drel.ModelMeta[Post]{
	Table: "posts",
}
`)

	if _, err := Generate(dir); err != nil {
		t.Fatalf("the generator failed: %v", err)
	}
	source := read(t, filepath.Join(dir, GeneratedFile))

	for _, want := range []string{
		"func (m *Module) Models() []module.ModelDesc {",
		`Name:  "Post"`,
		`Table: "posts"`,
		`{Name: "ID", Type: "uuid.UUID"}`,
		`{Name: "Title", Type: "string"}`,
		`{Name: "Body", Type: "string"}`,
		`{Name: "CreatedAt", Type: "time.Time"}`,
		`{Name: "UpdatedAt", Type: "time.Time"}`,
	} {
		if !strings.Contains(source, want) {
			t.Errorf("the generated file holds no %q\n%s", want, source)
		}
	}
}

// A feature slice with no model package writes no Models method.
func TestTheGeneratorWritesNoModelsWithoutAModelPackage(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, "handlers.go"), `package posts

import "github.com/alternayte/avero/router"

type Module struct{}

// Show renders one post.
func (m *Module) Show(c *router.Ctx, in ShowInput) (router.Response, error) {
	return nil, nil
}
`)
	write(t, filepath.Join(dir, "input.go"), `package posts

type ShowInput struct {
	ID string `+"`path:\"id\"`"+`
}
`)
	if _, err := Generate(dir); err != nil {
		t.Fatalf("the generator failed: %v", err)
	}
	if source := read(t, filepath.Join(dir, GeneratedFile)); strings.Contains(source, "Models()") {
		t.Fatalf("the generated file holds a Models method\n%s", source)
	}
}

// write puts one source file on the disk.
func write(t *testing.T, path, source string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(source), 0o600); err != nil {
		t.Fatal(err)
	}
}

// read returns the content of one file.
func read(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}
```

**Note:** `codegen/codegen_test.go` may already carry a helper with the name
`write` or `read`. Run
`grep -n "func write\|func read" codegen/*_test.go` first. If a helper exists,
delete the copy from this file and call the one that exists.

- [ ] **Step 2: Run the test and prove that it fails**

Run: `go test ./codegen -race -run TestTheGeneratorWritesThe -v`
Expected: FAIL, because the generated file holds no `Models` method.

- [ ] **Step 3: Add the `Models` field to the parsed package**

In `codegen/scan.go`, add this member to the `pkg` struct:

```go
	// Models holds the persistent models of the sibling model package, in
	// name order. It is empty for a package with no model directory.
	Models []model
```

- [ ] **Step 4: Write the model scan**

Create `codegen/model.go`:

```go
package codegen

import (
	"go/ast"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"
)

// The generator reads the model package of a feature slice at generation
// time. No generated file imports reflect, and no request path calls it. See
// the SDD, S5, and design rule 2.

// modelImport is the package that the generated Models method calls.
const modelImport = "github.com/alternayte/avero/module"

// model is one persistent model of a feature slice.
type model struct {
	// Name is the name of the Go type.
	Name string
	// Table is the name of the database table, which the meta value of drel
	// states.
	Table string
	// Fields holds the fields of the model: the identifier, then the
	// declared columns, then the two times.
	Fields []modelField
}

// modelField is one field of a model.
type modelField struct {
	// Name is the name of the Go field.
	Name string
	// Type is the type as source text, such as uuid.UUID.
	Type string
}

// modelsOf reads the model directory that stands beside dir and returns its
// models in name order. It returns nothing for a directory that is absent.
func modelsOf(fset *token.FileSet, dir string) ([]model, error) {
	path := filepath.Join(dir, "model")
	if info, err := os.Stat(path); err != nil || !info.IsDir() {
		return nil, nil
	}
	byPackage, err := parseDir(fset, path)
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(byPackage))
	for name := range byPackage {
		names = append(names, name)
	}
	sort.Strings(names)

	var out []model
	for _, name := range names {
		out = append(out, scanModels(fset, byPackage[name])...)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

// scanModels returns the models of one package.
//
// A model is a struct that embeds drel.Model. The type parameter of the
// embedded type states the type of the identifier, and the meta value of the
// same name states the table.
func scanModels(fset *token.FileSet, files []*ast.File) []model {
	tables := map[string]string{}
	structs := map[string]*ast.StructType{}

	for _, f := range files {
		for _, d := range f.Decls {
			gen, ok := d.(*ast.GenDecl)
			if !ok {
				continue
			}
			for _, spec := range gen.Specs {
				switch s := spec.(type) {
				case *ast.TypeSpec:
					if st, ok := s.Type.(*ast.StructType); ok {
						structs[s.Name.Name] = st
					}
				case *ast.ValueSpec:
					readTable(s, tables)
				}
			}
		}
	}

	var out []model
	for name, st := range structs {
		table, ok := tables[name]
		if !ok {
			continue
		}
		fields, ok := modelFields(fset, st)
		if !ok {
			continue
		}
		out = append(out, model{Name: name, Table: table, Fields: fields})
	}
	return out
}

// readTable records the table of a meta value, such as
// `var PostMeta = drel.ModelMeta[Post]{Table: "posts"}`.
func readTable(s *ast.ValueSpec, tables map[string]string) {
	for _, value := range s.Values {
		lit, ok := value.(*ast.CompositeLit)
		if !ok {
			continue
		}
		index, ok := lit.Type.(*ast.IndexExpr)
		if !ok || !isSelectorNamed(index.X, "ModelMeta") {
			continue
		}
		name, ok := index.Index.(*ast.Ident)
		if !ok {
			continue
		}
		for _, member := range lit.Elts {
			kv, ok := member.(*ast.KeyValueExpr)
			if !ok {
				continue
			}
			key, ok := kv.Key.(*ast.Ident)
			if !ok || key.Name != "Table" {
				continue
			}
			text, ok := kv.Value.(*ast.BasicLit)
			if !ok || text.Kind != token.STRING {
				continue
			}
			if table, err := strconv.Unquote(text.Value); err == nil {
				tables[name.Name] = table
			}
		}
	}
}

// modelFields returns the fields of one model, and it reports a struct that
// embeds drel.Model.
//
// The embedded type gives the identifier and the two times. Its type
// parameter states the type of the identifier.
func modelFields(fset *token.FileSet, st *ast.StructType) ([]modelField, bool) {
	var (
		key      string
		declared []modelField
	)
	for _, f := range st.Fields.List {
		if len(f.Names) == 0 {
			index, ok := f.Type.(*ast.IndexExpr)
			if ok && isSelectorNamed(index.X, "Model") {
				key = exprText(fset, index.Index)
			}
			continue
		}
		if f.Tag == nil {
			continue
		}
		if tag := reflect.StructTag(strings.Trim(f.Tag.Value, "`")); tag.Get("db") == "" {
			continue
		}
		typeText := exprText(fset, f.Type)
		for _, name := range f.Names {
			declared = append(declared, modelField{Name: name.Name, Type: typeText})
		}
	}
	if key == "" {
		return nil, false
	}
	fields := []modelField{{Name: "ID", Type: key}}
	fields = append(fields, declared...)
	fields = append(fields,
		modelField{Name: "CreatedAt", Type: "time.Time"},
		modelField{Name: "UpdatedAt", Type: "time.Time"})
	return fields, true
}
```

- [ ] **Step 5: Read the model directory in the build**

In `codegen/codegen.go`, in the `build` function, replace the two lines that
follow the call to `scan` with this:

```go
			p := scan(fset, name, byPackage[name], c)
			if len(p.Inputs) == 0 {
				continue
			}
			// The model package of the feature slice states the persistent
			// models. The generator writes them, so a person states each
			// field one time. See AN-3.
			if p.Receiver != "" {
				models, modelErr := modelsOf(fset, d)
				if modelErr != nil {
					return nil, modelErr
				}
				p.Models = models
			}
```

- [ ] **Step 6: Emit the method**

In `codegen/emit.go`, in the `emit` function, add the call after
`emitSummaries`:

```go
	emitModels(&body, p, imports)
```

Append this function to `codegen/emit.go`:

```go
// emitModels writes the Models method of the module of a package.
//
// The model package of the feature slice states each field, so a person
// writes it one time and the description of the module carries it. See
// module.ModelModule and AN-3.
func emitModels(b *strings.Builder, p pkg, imports map[string]bool) {
	if p.Receiver == "" || len(p.Models) == 0 {
		return
	}
	imports[modelImport] = true

	fmt.Fprintf(b, "// Models returns the persistent models of %s.\n", p.Receiver)
	b.WriteString("//\n")
	b.WriteString("// The model package of this feature states them. Change a field and run\n")
	b.WriteString("// `avero generate`.\n")
	fmt.Fprintf(b, "func (m *%s) Models() []module.ModelDesc {\n", p.Receiver)
	b.WriteString("\treturn []module.ModelDesc{\n")
	for _, mod := range p.Models {
		b.WriteString("\t\t{\n")
		fmt.Fprintf(b, "\t\t\tName:  %s,\n", strconv.Quote(mod.Name))
		fmt.Fprintf(b, "\t\t\tTable: %s,\n", strconv.Quote(mod.Table))
		b.WriteString("\t\t\tFields: []module.FieldDesc{\n")
		for _, f := range mod.Fields {
			fmt.Fprintf(b, "\t\t\t\t{Name: %s, Type: %s},\n",
				strconv.Quote(f.Name), strconv.Quote(f.Type))
		}
		b.WriteString("\t\t\t},\n\t\t},\n")
	}
	b.WriteString("\t}\n}\n\n")
}
```

- [ ] **Step 7: Run the test and prove that it passes**

Run: `go test ./codegen -race -run TestTheGeneratorWrites -v`
Expected: PASS

- [ ] **Step 8: Run the whole unit gate**

Run: `gofmt -l . && go vet ./... && golangci-lint run && go test ./... -race -count=1`
Expected: no output from `gofmt`, and PASS from the tests.

- [ ] **Step 9: Commit**

```bash
git add codegen/model.go codegen/model_test.go codegen/scan.go codegen/codegen.go codegen/emit.go
git commit -m "Write the Models method from the model package of a feature slice

Claude-Session: https://claude.ai/code/session_018m68TtjNW98aezz2u79Qiw"
```

---

### Task 7: The templates take the four changes

The examples are generated from the templates. This task changes every
template, writes the examples again, and proves that the three reference
applications still build and pass.

**Files:**
- Modify: `scaffold/templates/ssr/main.go.tmpl`
- Modify: `scaffold/templates/api/main.go.tmpl`
- Modify: `scaffold/templates/spa/main.go.tmpl`
- Modify: `scaffold/templates/ssr/wire.go.tmpl`
- Modify: `scaffold/templates/spa/wire.go.tmpl`
- Modify: `scaffold/templates/ssr/internal/features/posts/store.go.tmpl`
- Modify: `scaffold/templates/api/internal/features/posts/store.go.tmpl`
- Modify: `scaffold/templates/spa/internal/features/tasks/store.go.tmpl`
- Modify: `scaffold/templates/slice/store.go.tmpl`
- Modify: `scaffold/templates/ssr/internal/features/posts/module.go.tmpl`
- Modify: `scaffold/templates/api/internal/features/posts/module.go.tmpl`
- Modify: `scaffold/templates/spa/internal/features/tasks/module.go.tmpl`
- Modify: `scaffold/templates/slice/module.go.tmpl`

**Interfaces:**
- Consumes: `avero.Serve`, `avero.Service`, `avero.Repo`, `avero.Save`, `avero.MountAssets` and the generated `Models` method from Tasks 1, 2, 4 and 6.
- Produces: nothing that a later task reads.

- [ ] **Step 1: Read the current templates**

Run: `cat scaffold/templates/ssr/main.go.tmpl scaffold/templates/ssr/wire.go.tmpl`
Read the template actions, such as `{{ .Module }}`, and keep every one of them.

- [ ] **Step 2: Replace each `main.go.tmpl`**

Write the whole file. Keep the first comment line of the shape, and keep the
template action that states the module path.

```go
// Command {{ .Module }} is an Avero application of the ssr shape.
package main

import (
	"os"

	"github.com/alternayte/avero"
)

// main starts the application and returns the exit code of the process.
//
// Serve runs the sequence of the host: it answers an inspection command, it
// loads the configuration, it opens the database, it builds the router, it
// registers the boot checks and the migrator, and it serves until a signal.
// Read the doc comment of avero.Serve for the long form.
func main() {
	os.Exit(avero.Serve(avero.Service[Config]{
		Args:       os.Args[1:],
		Wire:       wire,
		DSN:        func(c Config) string { return c.DatabaseURL },
		Migrations: migrationSets,
	}))
}
```

Write the same file for the `api` shape and for the `spa` shape. Change only
the word `ssr` in the first line to `api` or to `spa`.

- [ ] **Step 3: Move `migrationSets` if the template needs it**

`migrationSets` stands in `wire.go.tmpl` today. Leave it there. `main.go`
names it, and both files hold the package `main`.

- [ ] **Step 4: Replace the asset block of `wire.go.tmpl`**

In `scaffold/templates/ssr/wire.go.tmpl` and in
`scaffold/templates/spa/wire.go.tmpl`, delete the call to
`avero.LoadManifest`, the call to `fs.Sub` and the call to `r.Mount`. Put this
in their place, at the position that the manifest call held:

```go
	// MountAssets reads the manifest and serves the built files at /assets/.
	// The binary carries them, so the server needs no directory beside it.
	manifest, err := avero.MountAssets(r, dist, "assets/dist")
	if err != nil {
		return nil, nil, err
	}
	ui.SetManifest(manifest)
```

`MountAssets` mounts the handler on the router, so the call must stand after
`avero.NewRouter`. Move the whole block below the call that builds `r`. The
`spa` shape holds no `ui` package, so delete the `ui.SetManifest` line there
and keep the rest.

Delete the import of `github.com/alternayte/avero/assets`, and delete the
import of `io/fs` if `migrationSets` no longer needs it. Keep `io/fs` when
`migrationSets` states the type `[]fs.FS`.

- [ ] **Step 5: Replace the store helpers**

In each `store.go.tmpl`, delete the unexported `repo` method and the
unexported `save` method. Change every call site:

- `s.repo(ctx)` becomes `avero.Repo(ctx, model.PostMeta)`, with the meta value
  of the shape. The `spa` shape states `model.TaskMeta`.
- `s.save(ctx)` becomes `avero.Save(ctx)`.

Add the import `github.com/alternayte/avero`. Delete the import of `errors` if
no other line uses it.

The comment of the `Store` type keeps the sentence about the transaction of
the request. Change its last sentence to name the new helper:

```go
// Every call runs inside the transaction of the request, which the transaction
// middleware opens. avero.Repo returns the repository of that transaction, and
// avero.Save flushes the staged changes. See S4.
```

- [ ] **Step 6: Delete the `Describe` method**

In each `module.go.tmpl`, delete the whole `Describe` method. The generator
writes the `Models` method, and the module system builds the description from
the routes and the models. Add this comment above the `Routes` method:

```go
// The description of this module needs no hand-written method. `avero
// generate` writes the models from the model package, and the module system
// reads the routes from this method. See AN-3.
```

- [ ] **Step 7: Write the examples again**

Run: `just examples`
Then run: `go generate ./...`
Expected: the three example directories change, and each `zz_generated.go`
gains a `Models` method.

- [ ] **Step 8: Prove that the examples build and pass**

Run:
```bash
go run ./internal/cmd/averoexamples -check
go run ./internal/cmd/averoexamples -assets
cd examples/blog && go build ./... && go test ./... -count=1 && cd ../..
cd examples/board && go build ./... && go test ./... -count=1 && cd ../..
cd examples/orders && go build ./... && go test ./... -count=1 && cd ../..
```
Expected: PASS for each application.

- [ ] **Step 9: Measure the result**

Run:
```bash
cd examples/blog && find . -name '*.go' ! -name '*_gen.go' ! -name 'zz_*' \
  ! -name '*_drel.go' ! -name '*_templ.go' ! -name '*_test.go' | xargs wc -l
```
Expected: about 390 lines in place of 532. Record the number. If the total is
above 430, one change did not land. Read the diff of the templates again.

- [ ] **Step 10: Run the whole gate**

Run: `just verify`
Expected: every step passes.

- [ ] **Step 11: Commit**

```bash
git add scaffold/templates examples
git commit -m "Write the templates with Serve, Repo, Save and MountAssets

Claude-Session: https://claude.ai/code/session_018m68TtjNW98aezz2u79Qiw"
```

---

### Task 8: The documents state the new form

**Files:**
- Modify: `docs/getting-started.md`
- Modify: `docs/agents.md`
- Modify: `docs/configuration.md`
- Modify: `docs/internal/sdd.md`
- Modify: `scaffold/templates/agents/AGENTS.md.tmpl`
- Modify: `scaffold/templates/agents/claude-skills/add-slice/SKILL.md.tmpl`
- Modify: `CHANGELOG.md`

**Interfaces:**
- Consumes: every export of Tasks 1, 2, 4, 5 and 6.
- Produces: nothing that a later task reads.

- [ ] **Step 1: Find every stale passage**

Run:
```bash
grep -rn "avero.Inspecting\|LoadManifest\|func (s \*Store) repo\|Describe() avero.Description" \
  docs scaffold/templates README.md CHANGELOG.md
```
Every hit is a passage to change.

- [ ] **Step 2: Change `docs/getting-started.md`**

Replace the `main.go` listing with the `avero.Serve` form of Task 7, Step 2.
Add one paragraph after it:

```markdown
`avero.Serve` runs the start sequence. It answers `avero routes` and `avero
doctor`, it loads the configuration, it opens the database, it builds the
router, it registers the boot checks and the migrator, and it serves until a
signal. An application with an unusual start calls `avero.Load`, `avero.New`
and `Run` itself.
```

- [ ] **Step 3: Change `docs/agents.md`**

Replace the passage that tells an agent to write a `Describe` method. State
this instead:

```markdown
A module needs no `Describe` method. `avero generate` reads the `model`
package of the feature slice and writes a `Models` method. The module system
builds the description from the routes and the models. Run `avero schema
--json` to read the result.
```

- [ ] **Step 4: Change `docs/internal/sdd.md`**

In section 5.2, add `ModelModule` to the list of optional module interfaces:

```go
type ModelModule     interface { Models() []ModelDesc }        // AN-3, generated
```

In section 5.3, add one sentence before the numbered list:

```markdown
`avero.Serve` runs this sequence. An application that calls `avero.New`
directly runs it itself.
```

- [ ] **Step 5: Change the scaffolded agent files**

In `scaffold/templates/agents/claude-skills/add-slice/SKILL.md.tmpl`, delete
the step that writes a `Describe` method, and change the store step to name
`avero.Repo` and `avero.Save`.

In `scaffold/templates/agents/AGENTS.md.tmpl`, change the same two points.

- [ ] **Step 6: Add the changelog entry**

Add this block under the unreleased heading of `CHANGELOG.md`:

```markdown
### Added

- `avero.Serve` runs the start sequence, so `main.go` states the application
  and not the sequence.
- `avero.Repo` and `avero.Save` return the repository of the transaction of
  the request, so a store states no lookup.
- `avero.MountAssets` reads the manifest and serves the built assets.
- `avero generate` writes a `Models` method from the `model` package of a
  feature slice. `module.ModelModule` states the contract.

### Changed

- A scaffolded module states no `Describe` method. The module system builds
  the description from the routes and the generated models.
- A scaffolded application writes about 390 lines of Go in place of 532.
```

- [ ] **Step 7: Write the examples again and run the gate**

Run:
```bash
just examples
just verify
```
Expected: every step passes.

- [ ] **Step 8: Commit**

```bash
git add docs scaffold/templates CHANGELOG.md examples
git commit -m "State Serve, Repo, Save and the generated models in the documents

Claude-Session: https://claude.ai/code/session_018m68TtjNW98aezz2u79Qiw"
```

---

## Verification

The work is complete when each of these holds:

- [ ] `just verify` passes with no failed step.
- [ ] `examples/blog` holds about 390 hand-written lines of Go.
- [ ] `examples/blog/main.go` holds 20 lines or fewer.
- [ ] `avero routes` and `avero doctor` still run on the three examples.
- [ ] `avero schema --json` names the model `Post` with its five fields.
- [ ] DX-5 holds: a cold `go build` of a scaffolded SSR application takes 15 s or less. Read `artifacts/verification.md`.
