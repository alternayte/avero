# Wiring Extensibility Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Let a user add a dependency that Avero does not carry, with a lifecycle and a boot check, and keep the lean `main.go` that `avero.Serve` gave them.

**Architecture:** `wire` returns one `*avero.Wiring` value in place of three. The struct carries the router, the module set, the components and the boot checks. `Serve` reads all four, so a value that `wire` built reaches the lifecycle and the doctor. `avero doctor` loads the configuration and reads the database address from it, which retires the `DSNEnv` workaround. A module states its own migrations through `MigrationModule`, which retires `Service.Migrations` and `migrationSets`.

**Tech Stack:** Go 1.24, `github.com/alternayte/drel` v0.7.1, `just` for the gate.

**Spec:** `docs/superpowers/specs/2026-09-10-wiring-extensibility-design.md`

## Global Constraints

- Module path is `github.com/alternayte/avero`. The binary is `avero`.
- Write every comment, document and commit message in ASD-STE100 Simplified Technical English: active voice, one word for one meaning, no contraction, no `-ing` form except in a technical name, no `e.g.`/`i.e.`/`etc.`, descriptive sentences of 25 words or fewer and procedural sentences of 20 words or fewer, three nouns together at most, no slash between words, no parentheses that add information, no dash that joins two clauses.
- Never edit a file under `examples/` by hand. Change `scaffold/templates`, then run `just examples`.
- Never edit a `zz_generated.go` file by hand. Change the generator.
- Write the test before the code that it judges.
- Never relax a test to make the gate pass. Never skip a test.
- No global mutable state. No facade. No package-level singleton.
- Dependencies are struct fields. The constructor takes them.
- Explicit beats short. Do not add magic to save a line.
- An error carries one sentence that states the repair. A fault appears before the process starts.
- Move a fault to compile time when you can.
- Every new export carries a doc comment.
- The work starts from the `lean-user-code` branch. `avero.Serve` lives there and is not on `main`.
- The gate is `just verify`. A task is complete when `gofmt -l .` prints nothing and `go vet ./...`, `golangci-lint run` and `go test ./... -race -count=1` all pass.
- End each commit message with a blank line and then `Claude-Session: https://claude.ai/code/session_018m68TtjNW98aezz2u79Qiw`.

---

## File Structure

**Create:**

- `wiring.go` — the `Wiring` type and its validation. One responsibility: state what `wire` produces.
- `wiring_test.go` — the tests of the validation.

**Modify:**

- `serve.go` — `Service.Wire` takes the new shape. `Serve` reads the wiring. The inspect path splits. `Service.Components`, `Service.Migrations` and `Service.DSNEnv` go away. `fsMigrator` holds a slice and not a function.
- `serve_test.go` — the existing tests take the new `wire` shape, and the new behaviour gains tests.
- `avero.go` — the alias block gains `Wiring`.
- `scaffold/templates/{ssr,api,spa}/wire.go.tmpl` — `wire` returns `*avero.Wiring`. `migrationSets` goes away.
- `scaffold/templates/{ssr,api,spa}/main.go.tmpl` — the `Migrations` field goes away.
- `scaffold/templates/{ssr,api,spa}/acceptance_test.go.tmpl` — the test reads the migrations from the module set.
- `scaffold/templates/{ssr,api,spa}/internal/features/*/module.go.tmpl` — each module states `Migrations`.
- `scaffold/templates/slice/module.go.tmpl` — the same method.
- `docs/getting-started.md`, `docs/configuration.md`, `docs/agents.md`, `docs/internal/sdd.md`, `CHANGELOG.md`.
- `scaffold/templates/agents/claude-skills/add-slice/SKILL.md.tmpl` — the step that adds a slice states the `Migrations` method.

---

### Task 1: The Wiring type and the new Wire shape

`wire` returns three values and an error today. This task replaces them with
one struct, so a later field breaks nobody and an early error returns two
values.

**Files:**
- Create: `wiring.go`
- Create: `wiring_test.go`
- Modify: `serve.go`
- Modify: `serve_test.go`
- Modify: `avero.go`

**Interfaces:**
- Consumes: `Router`, `ModuleSet`, `Component` and `Check`, which `avero.go` already exports.
- Produces:
  - `type Wiring struct { Router *Router; Modules *ModuleSet; Components []Component; Checks []Check }`
  - `func (w *Wiring) validate() error`
  - `Service.Wire func(engine *drel.Engine, cfg C) (*Wiring, error)`

- [ ] **Step 1: Write the failing test**

Create `wiring_test.go`:

```go
package avero

import "testing"

// A wiring that states no router names the repair, so a person reads what to
// set. See DX-7.
func TestTheWiringStatesItsFaults(t *testing.T) {
	cases := []struct {
		name  string
		w     *Wiring
		holds string
	}{
		{"a nil wiring", nil, "*avero.Wiring"},
		{"no router", &Wiring{Modules: Modules()}, "Router"},
		{"no module set", &Wiring{Router: NewRouter()}, "Modules"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := c.w.validate()
			if err == nil {
				t.Fatal("validate returned no error")
			}
			if !strings.Contains(err.Error(), c.holds) {
				t.Fatalf("the error does not name %q: %v", c.holds, err)
			}
		})
	}
}

// A whole wiring passes.
func TestTheWiringPasses(t *testing.T) {
	w := &Wiring{Router: NewRouter(), Modules: Modules()}
	if err := w.validate(); err != nil {
		t.Fatalf("validate failed: %v", err)
	}
}
```

Add the import `strings`. This test file is in package `avero` and not in
`avero_test`, because `validate` is unexported.

- [ ] **Step 2: Run the test and prove that it fails**

Run: `go test . -race -run TestTheWiring -v`
Expected: FAIL, because `Wiring` does not exist.

- [ ] **Step 3: Write the type**

Create `wiring.go`:

```go
package avero

import "errors"

// Wiring is what wire builds. Serve reads it.
//
// The Router and the Modules are required. The Components and the Checks are
// optional, so an application that adds no dependency of its own states two
// fields.
type Wiring struct {
	// Router holds the routes of the application.
	Router *Router
	// Modules holds the module set. Serve reads the migrations of each
	// module from it.
	Modules *ModuleSet
	// Components are the parts of the application that hold a lifecycle,
	// such as an emailer, a blob store or a cache. Serve starts them in this
	// order and stops them in reverse order.
	//
	// A constructor must not dial, connect or read a file, because wire also
	// runs for an inspection command with a nil engine and a zero
	// configuration. Build the value in the constructor. Connect in Start.
	Components []Component
	// Checks are the boot checks of the application. Serve runs them before
	// the process serves, and `avero doctor` reports them. See DX-8.
	Checks []Check
}

// validate reports a wiring that Serve cannot use. Each fault states the
// repair. See DX-7.
func (w *Wiring) validate() error {
	switch {
	case w == nil:
		return errors.New("wire returned no wiring: return a *avero.Wiring that holds its Router and its Modules")
	case w.Router == nil:
		return errors.New("the wiring holds no router: set the Router field to the value that avero.NewRouter returned")
	case w.Modules == nil:
		return errors.New("the wiring holds no module set: set the Modules field to the value that avero.Modules returned")
	}
	return nil
}
```

- [ ] **Step 4: Run the test and prove that it passes**

Run: `go test . -race -run TestTheWiring -v`
Expected: PASS

- [ ] **Step 5: Change the Wire field**

`Wiring` lives in the root package already, so `avero.go` needs no alias for
it. Add none.



In `serve.go`, change the `Wire` field of `Service`:

```go
	// Wire builds the wiring of the application. An inspection command
	// passes a nil engine, and every command but `avero doctor` passes a
	// zero configuration.
	Wire func(engine *drel.Engine, cfg C) (*Wiring, error)
```

Delete the `Components` field of `Service`. `Wiring.Components` replaces it.

- [ ] **Step 6: Change Serve to read the wiring**

In `serve.go`, in the run path of `Serve`, replace the three lines that call
`s.Wire` and build the handler with:

```go
	w, err := s.Wire(engine, *cfg)
	if err != nil {
		return Exit(errOut, err)
	}
	if err := w.validate(); err != nil {
		return Exit(errOut, err)
	}
	handler, err := w.Router.Handler()
	if err != nil {
		return Exit(errOut, err)
	}
```

Delete the comment block that begins "The module set carries the jobs".

In the inspect path, replace the call with:

```go
		w, err := s.Wire(nil, zero)
		if err != nil {
			return Exit(errOut, err)
		}
		if err := w.validate(); err != nil {
			return Exit(errOut, err)
		}
		return Inspect[C](s.Args, out, errOut, w.Router, w.Modules, s.doctorChecks()...)
```

Delete the block that reads `s.Components`. Task 2 puts the components back
through the wiring.

- [ ] **Step 7: Change the tests to the new shape**

In `serve_test.go`, change `serveWire` and every other wire function:

```go
// serveWire builds a wiring with one route and no module.
func serveWire(_ *drel.Engine, _ serveConfig) (*avero.Wiring, error) {
	r := avero.NewRouter()
	r.Get("/{$}", func(c *avero.Ctx) (avero.Response, error) {
		return avero.Text(http.StatusOK, "ready"), nil
	})
	return &avero.Wiring{Router: r, Modules: avero.Modules()}, nil
}
```

Delete the test that proves `Service.Components` reaches the application.
Task 2 writes its replacement.

Add one test that proves a wiring fault stops the run:

```go
// A wire that returns a wiring with no router stops the run and names the
// repair.
func TestServeStopsOnAWiringFault(t *testing.T) {
	t.Setenv("AVERO_SECRET", strings.Repeat("a", 64))
	var errOut strings.Builder
	code := avero.Serve(avero.Service[serveConfig]{
		Wire: func(*drel.Engine, serveConfig) (*avero.Wiring, error) {
			return &avero.Wiring{Modules: avero.Modules()}, nil
		},
		Out: io.Discard,
		Err: &errOut,
	})
	if code != 1 {
		t.Fatalf("the run returned the code %d", code)
	}
	if !strings.Contains(errOut.String(), "Router") {
		t.Fatalf("the fault does not name the field: %q", errOut.String())
	}
}
```

- [ ] **Step 8: Run the whole unit gate**

Run: `gofmt -l . && go vet ./... && golangci-lint run && go test . ./module ./config -race -count=1`
Expected: no output from `gofmt`, and PASS from the tests.

The examples do not build yet, because their `wire` still returns three
values. Task 5 repairs them. Do not run `just verify` in this task, and do not
run `go build ./...` over the examples.

- [ ] **Step 9: Commit**

```bash
git add wiring.go wiring_test.go serve.go serve_test.go
git commit -m "Return one Wiring from wire, so a later part breaks no signature

Claude-Session: https://claude.ai/code/session_018m68TtjNW98aezz2u79Qiw"
```

---

### Task 2: The components and the checks of the wiring reach the application

This is the repair that the whole plan exists for. A user builds an emailer in
`wire` and states it in the wiring. `Serve` starts it, stops it, and runs its
boot check.

**Files:**
- Modify: `serve.go`
- Modify: `serve_test.go`

**Interfaces:**
- Consumes: `Wiring` from Task 1.
- Produces: no new export. `Serve` registers `w.Components` and appends `w.Checks`.

- [ ] **Step 1: Write the failing tests**

Add to `serve_test.go`:

```go
// recorder is a component that records its own lifecycle. A test proves the
// order of a start and of a stop.
type recorder struct {
	name string
	log  *[]string
}

func (r recorder) Name() string { return r.name }

func (r recorder) Start(context.Context) error {
	*r.log = append(*r.log, "start "+r.name)
	return nil
}

func (r recorder) Stop(context.Context) error {
	*r.log = append(*r.log, "stop "+r.name)
	return nil
}

// A component that wire builds starts with the application and stops with it.
// Avero starts in registration order and stops in reverse order.
func TestServeStartsTheComponentsOfTheWiring(t *testing.T) {
	t.Setenv("AVERO_SECRET", strings.Repeat("a", 64))
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("the listener does not open: %v", err)
	}

	var log []string
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan int, 1)
	go func() {
		done <- avero.Serve(avero.Service[serveConfig]{
			Wire: func(*drel.Engine, serveConfig) (*avero.Wiring, error) {
				r := avero.NewRouter()
				r.Get("/{$}", func(c *avero.Ctx) (avero.Response, error) {
					return avero.Text(http.StatusOK, "ready"), nil
				})
				return &avero.Wiring{
					Router:  r,
					Modules: avero.Modules(),
					Components: []avero.Component{
						recorder{name: "first", log: &log},
						recorder{name: "second", log: &log},
					},
				}, nil
			},
			Ctx:     ctx,
			Out:     io.Discard,
			Err:     io.Discard,
			Options: []avero.Option{avero.WithListener(listener), avero.WithoutSignals()},
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

	want := []string{"start first", "start second", "stop second", "stop first"}
	if !slices.Equal(log, want) {
		t.Fatalf("the lifecycle reads %v and it must read %v", log, want)
	}
}

// A check that the wiring carries stops a bad boot with the code 1, and the
// fault names the repair. See DX-8.
func TestServeRunsTheChecksOfTheWiring(t *testing.T) {
	t.Setenv("AVERO_SECRET", strings.Repeat("a", 64))
	var errOut strings.Builder
	code := avero.Serve(avero.Service[serveConfig]{
		Wire: func(*drel.Engine, serveConfig) (*avero.Wiring, error) {
			return &avero.Wiring{
				Router:  avero.NewRouter(),
				Modules: avero.Modules(),
				Checks: []avero.Check{{
					Name:   "the mail server",
					Repair: "Set SMTP_URL to the address of the mail server",
					Run: func(context.Context) error {
						return errors.New("the mail server does not answer")
					},
				}},
			}, nil
		},
		Out: io.Discard,
		Err: &errOut,
	})
	if code != 1 {
		t.Fatalf("the run returned the code %d", code)
	}
	if !strings.Contains(errOut.String(), "SMTP_URL") {
		t.Fatalf("the fault does not state the repair: %q", errOut.String())
	}
}
```

Add the imports `errors`, `net` and `slices` if they are absent.

- [ ] **Step 2: Run the tests and prove that they fail**

Run: `go test . -race -run 'TestServeStartsTheComponents|TestServeRunsTheChecks' -v`
Expected: FAIL. The first test records no lifecycle. The second exits 0.

- [ ] **Step 3: Register the components and append the checks**

In `serve.go`, in `Serve`, append the checks of the wiring after the checks
that `Serve` builds:

```go
	// The checks of the application run after the checks of Avero, so the
	// secret, the database and the migrations are proved first.
	checks = append(checks, w.Checks...)
```

Then register the components, before the line that appends `s.Options`:

```go
	if len(w.Components) > 0 {
		opts = append(opts, WithComponents(w.Components...))
	}
```

- [ ] **Step 4: Run the tests and prove that they pass**

Run: `go test . -race -run 'TestServeStartsTheComponents|TestServeRunsTheChecks' -v`
Expected: PASS

- [ ] **Step 5: Run the unit gate**

Run: `gofmt -l . && go vet ./... && golangci-lint run && go test . -race -count=1`
Expected: no output from `gofmt`, and PASS.

- [ ] **Step 6: Commit**

```bash
git add serve.go serve_test.go
git commit -m "Start the components of the wiring, and run its checks

Claude-Session: https://claude.ai/code/session_018m68TtjNW98aezz2u79Qiw"
```

---

### Task 3: The doctor reads the address from the configuration

`Serve` reads the database address with `os.Getenv` for `avero doctor`, so an
application that composes its address gets a doctor that reports success and
proves nothing. `Inspect` already loads the configuration for that command.

**Files:**
- Modify: `serve.go`
- Modify: `serve_test.go`

**Interfaces:**
- Consumes: `Wiring` from Task 1, `InspectPrefix` and `Inspect` from `inspect.go`.
- Produces:
  - `func (s Service[C]) inspect(ctx context.Context, out, errOut io.Writer) int`
  - `func (s Service[C]) doctorChecks(cfg C, w *Wiring) []Check`
  - `Service.DSNEnv` is deleted.

- [ ] **Step 1: Write the failing tests**

Add to `serve_test.go`:

```go
// `avero doctor` reports a check that the wiring carries, so a person proves
// a dependency of the application before the process serves. See DX-8.
func TestTheDoctorReportsTheChecksOfTheWiring(t *testing.T) {
	t.Setenv("AVERO_SECRET", strings.Repeat("a", 64))
	var out strings.Builder
	code := avero.Serve(avero.Service[serveConfig]{
		Args: []string{avero.InspectPrefix + "doctor"},
		Wire: func(*drel.Engine, serveConfig) (*avero.Wiring, error) {
			return &avero.Wiring{
				Router:  avero.NewRouter(),
				Modules: avero.Modules(),
				Checks: []avero.Check{{
					Name:   "the mail server",
					Repair: "Set SMTP_URL to the address of the mail server",
					Run:    func(context.Context) error { return nil },
				}},
			}, nil
		},
		Out: &out,
		Err: io.Discard,
	})
	if code != 0 {
		t.Fatalf("the doctor returned the code %d", code)
	}
	if !strings.Contains(out.String(), "the mail server") {
		t.Fatalf("the doctor does not name the check: %q", out.String())
	}
}

// The doctor reads the address from the configuration and not from the
// process environment, so an application that composes its address gets a
// real database check.
func TestTheDoctorReadsTheAddressFromTheConfiguration(t *testing.T) {
	t.Setenv("AVERO_SECRET", strings.Repeat("a", 64))
	t.Setenv("DATABASE_URL", "")
	var out strings.Builder
	code := avero.Serve(avero.Service[serveConfig]{
		Args: []string{avero.InspectPrefix + "doctor"},
		Wire: serveWire,
		DSN: func(serveConfig) string {
			return "file:" + filepath.Join(t.TempDir(), "doctor.db")
		},
		Out: &out,
		Err: io.Discard,
	})
	if code != 0 {
		t.Fatalf("the doctor returned the code %d", code)
	}
	if !strings.Contains(out.String(), "database") {
		t.Fatalf("the doctor states no database check: %q", out.String())
	}
}

// An inspection command that is not the doctor reads no configuration, so
// `avero routes` works on a machine with no database. See DX-8.
func TestTheRoutesCommandReadsNoConfiguration(t *testing.T) {
	t.Setenv("AVERO_SECRET", "")
	var out strings.Builder
	code := avero.Serve(avero.Service[serveConfig]{
		Args: []string{avero.InspectPrefix + "routes"},
		Wire: serveWire,
		DSN:  func(serveConfig) string { return "file:/does/not/exist/x.db" },
		Out:  &out,
		Err:  io.Discard,
	})
	if code != 0 {
		t.Fatalf("the routes command returned the code %d", code)
	}
}
```

Add the import `path/filepath` if it is absent.

**Note on the second test:** read `host.Doctor` and the report that it writes
before you assert on the word `database`. Use the word that the report really
prints for a database check. Change the assertion to match, and say so in your
report.

- [ ] **Step 2: Run the tests and prove that they fail**

Run: `go test . -race -run 'TestTheDoctor|TestTheRoutesCommand' -v`
Expected: FAIL. The doctor reports no check of the wiring, and it reads no
address from the configuration.

- [ ] **Step 3: Split the inspect path**

In `serve.go`, replace the whole `if Inspecting(s.Args)` block of `Serve`
with:

```go
	if Inspecting(s.Args) {
		return s.inspect(ctx, out, errOut)
	}
```

Add these three functions to `serve.go`:

```go
// inspect answers one inspection command.
//
// `avero doctor` reports the checks of the application, so it loads the
// configuration. Every other command reads none, so `avero routes` works on a
// machine with no database. See DX-8.
//
// wire receives a nil engine in both cases. A route registration touches no
// database, and a boot check opens its own connection.
func (s Service[C]) inspect(ctx context.Context, out, errOut io.Writer) int {
	var cfg C
	doctor := isDoctor(s.Args)
	if doctor {
		// A configuration that does not load does not stop the doctor.
		// Inspect reports the fault itself.
		if loaded, err := Load[C](ctx); err == nil {
			cfg = *loaded
		}
	}

	w, err := s.Wire(nil, cfg)
	if err != nil {
		return Exit(errOut, err)
	}
	if err := w.validate(); err != nil {
		return Exit(errOut, err)
	}

	var checks []Check
	if doctor {
		checks = s.doctorChecks(cfg, w)
	}
	return Inspect[C](s.Args, out, errOut, w.Router, w.Modules, checks...)
}

// isDoctor reports the doctor command.
func isDoctor(args []string) bool {
	return len(args) > 0 && strings.TrimPrefix(args[0], InspectPrefix) == "doctor"
}

// doctorChecks returns the boot checks that `avero doctor` runs.
//
// The doctor holds no engine, so each check opens its own connection and
// closes it. The boot uses the engine of the application instead. See DX-8.
func (s Service[C]) doctorChecks(cfg C, w *Wiring) []Check {
	if s.DSN == nil {
		return w.Checks
	}
	dsn := s.DSN(cfg)
	if dsn == "" {
		return w.Checks
	}
	checks := []Check{DatabaseCheck(dsn)}
	if sets := w.Modules.Migrations(); len(sets) > 0 {
		checks = append(checks, MigrationCheckFS(dsn, sets...))
	}
	return append(checks, w.Checks...)
}
```

Add the import `strings`.

**Note:** `doctorChecks` reads `w.Modules.Migrations()`. That returns an empty
set until Task 5 makes each module state its migrations. It is correct now and
it becomes useful then.

- [ ] **Step 4: Delete DSNEnv**

Delete the `DSNEnv` field of `Service`, and delete the `dsnEnv` method.

In the fault message of the database open, name the variable plainly:

```go
			_, _ = fmt.Fprintf(errOut,
				"the database does not open: %v\n  → Prove the database address in the configuration. Start the database.\n",
				err)
```

Change any test that asserts on the old wording.

- [ ] **Step 5: Run the tests and prove that they pass**

Run: `go test . -race -run 'TestTheDoctor|TestTheRoutesCommand|TestServe' -v`
Expected: PASS

- [ ] **Step 6: Run the unit gate**

Run: `gofmt -l . && go vet ./... && golangci-lint run && go test . -race -count=1`
Expected: no output from `gofmt`, and PASS.

- [ ] **Step 7: Commit**

```bash
git add serve.go serve_test.go
git commit -m "Read the database address of the doctor from the configuration

Claude-Session: https://claude.ai/code/session_018m68TtjNW98aezz2u79Qiw"
```

---

### Task 4: A module states its own migrations

`Service.Migrations` and `migrationSets` in `wire.go` state a list that the
module set already holds. This task makes `Serve` read the module set.

**Files:**
- Modify: `serve.go`
- Modify: `serve_test.go`

**Interfaces:**
- Consumes: `module.Set.Migrations() []fs.FS`, which `module/inspect.go:251` already exports.
- Produces:
  - `Service.Migrations` is deleted.
  - `fsMigrator` holds `sets []fs.FS` in place of `sets func() []fs.FS`.

- [ ] **Step 1: Write the failing test**

Add to `serve_test.go`:

```go
// migrating is a module that carries migration files, so Serve reads the set
// from the module set and not from a second list.
type migrating struct{ files fs.FS }

func (migrating) Name() string { return "migrating" }

func (m migrating) Migrations() fs.FS { return m.files }

// Serve applies the migrations that a module states, so an application keeps
// one list and not two.
func TestServeAppliesTheMigrationsOfAModule(t *testing.T) {
	t.Setenv("AVERO_SECRET", strings.Repeat("a", 64))
	t.Setenv("MIGRATE_ON_BOOT", "true")
	dsn := "file:" + filepath.Join(t.TempDir(), "migrate.db")
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("the listener does not open: %v", err)
	}

	files := fstest.MapFS{
		"0001_widgets.sql": &fstest.MapFile{
			Data: []byte("CREATE TABLE widgets (id TEXT PRIMARY KEY);"),
		},
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan int, 1)
	go func() {
		done <- avero.Serve(avero.Service[serveConfig]{
			Wire: func(*drel.Engine, serveConfig) (*avero.Wiring, error) {
				return &avero.Wiring{
					Router:  avero.NewRouter(),
					Modules: avero.Modules(migrating{files: files}),
				}, nil
			},
			DSN:     func(serveConfig) string { return dsn },
			Ctx:     ctx,
			Out:     io.Discard,
			Err:     io.Discard,
			Options: []avero.Option{avero.WithListener(listener), avero.WithoutSignals()},
		})
	}()

	time.Sleep(500 * time.Millisecond)
	cancel()

	select {
	case code := <-done:
		if code != 0 {
			t.Fatalf("the run returned the code %d", code)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("the run did not stop")
	}

	// No migration is pending, so the migrator read the set of the module
	// and applied it. MigrationCheckOnFS states the same fact that the boot
	// check states, so the test needs no raw query.
	engine, err := drel.NewEngine(dsn)
	if err != nil {
		t.Fatalf("the database does not open: %v", err)
	}
	defer engine.Close()
	check := avero.MigrationCheckOnFS(engine, files)
	if err := check.Run(context.Background()); err != nil {
		t.Fatalf("a migration is still pending: %v", err)
	}
}
```

**Note:** read the `Check` type in `host/component.go` before you write this.
It holds `Name`, `Repair` and `Run`. Call the `Run` field.

Add the imports `io/fs`, `testing/fstest`, `net` and `path/filepath` if they
are absent.

The name of the migration file must match the pattern that drel expects. Read
`examples/blog/internal/features/posts/migrations/` for a real file name, and
use that shape.

- [ ] **Step 2: Run the test and prove that it fails**

Run: `go test . -race -run TestServeAppliesTheMigrationsOfAModule -v`
Expected: FAIL. The table is absent, because `Serve` reads
`s.Migrations` and the test states none.

- [ ] **Step 3: Read the sets from the module set**

In `serve.go`, in `Serve`, after the wiring is validated, read the sets one
time:

```go
	// A module states its own migration files. The module set merges them in
	// registration order, so an application keeps one list and not two.
	sets := w.Modules.Migrations()
```

Change the migration check:

```go
		if !base.MigrateOnBoot && len(sets) > 0 {
			checks = append(checks, MigrationCheckOnFS(engine, sets...))
		}
```

Change the migrator:

```go
	if engine != nil && len(sets) > 0 {
		opts = append(opts, WithMigrator(fsMigrator{engine: engine, sets: sets}))
	}
```

Change `fsMigrator`:

```go
// fsMigrator applies the pending migrations when MIGRATE_ON_BOOT is true.
//
// Each feature slice carries its own migrations, and drel merges the sets in
// version order. The files come from the binary, so one artifact carries the
// server and the schema.
type fsMigrator struct {
	engine *drel.Engine
	sets   []fs.FS
}

// Migrate applies every migration that the database does not hold.
func (m fsMigrator) Migrate(ctx context.Context) error {
	_, err := m.engine.ApplyMigrationsFS(ctx, m.sets...)
	return err
}
```

- [ ] **Step 4: Delete the Migrations field**

Delete the `Migrations` field of `Service`. Change the example in the doc
comment of `Serve` to state three fields:

```go
//	func main() {
//	    os.Exit(avero.Serve(avero.Service[Config]{
//	        Args: os.Args[1:],
//	        Wire: wire,
//	        DSN:  func(c Config) string { return c.DatabaseURL },
//	    }))
//	}
```

Change every test that states the `Migrations` field. A test that proved the
migration check must now state a module that carries files.

- [ ] **Step 5: Run the tests and prove that they pass**

Run: `go test . -race -run TestServe -v`
Expected: PASS

- [ ] **Step 6: Run the unit gate**

Run: `gofmt -l . && go vet ./... && golangci-lint run && go test . -race -count=1`
Expected: no output from `gofmt`, and PASS.

- [ ] **Step 7: Commit**

```bash
git add serve.go serve_test.go
git commit -m "Read the migration sets from the module set, so one list stands

Claude-Session: https://claude.ai/code/session_018m68TtjNW98aezz2u79Qiw"
```

---

### Task 5: The templates take the new shape

The four earlier tasks changed the framework. The examples do not build until
this task lands. Do not stop before the gate passes.

**Files:**
- Modify: `scaffold/templates/ssr/wire.go.tmpl`
- Modify: `scaffold/templates/api/wire.go.tmpl`
- Modify: `scaffold/templates/spa/wire.go.tmpl`
- Modify: `scaffold/templates/ssr/main.go.tmpl`
- Modify: `scaffold/templates/api/main.go.tmpl`
- Modify: `scaffold/templates/spa/main.go.tmpl`
- Modify: `scaffold/templates/ssr/acceptance_test.go.tmpl`
- Modify: `scaffold/templates/api/acceptance_test.go.tmpl`
- Modify: `scaffold/templates/spa/acceptance_test.go.tmpl`
- Modify: `scaffold/templates/ssr/internal/features/posts/module.go.tmpl`
- Modify: `scaffold/templates/api/internal/features/posts/module.go.tmpl`
- Modify: `scaffold/templates/spa/internal/features/tasks/module.go.tmpl`
- Modify: `scaffold/templates/slice/module.go.tmpl`

**Interfaces:**
- Consumes: `avero.Wiring` from Task 1, and the `Serve` of Tasks 2 to 4.
- Produces: nothing that a later task reads.

- [ ] **Step 1: Read the real templates**

Run: `cat scaffold/templates/ssr/wire.go.tmpl scaffold/templates/ssr/main.go.tmpl scaffold/templates/ssr/acceptance_test.go.tmpl`
Keep every template action, such as `{{ .Module }}`, exactly as the file
writes it. Do not invent one.

- [ ] **Step 2: Each module states its migrations**

In each of the four `module.go.tmpl` files, add this method after `Routes`:

```go
// Migrations returns the migration files of this feature. The module set
// merges the sets of every module, and drel applies them in version order.
func (m *Module) Migrations() fs.FS { return migrations.FS }
```

Add the import `io/fs`, and the import of the migrations package of the
slice. Read the `wire.go.tmpl` of the same shape for the import path and the
alias that it already uses, such as
`postmigrations "{{ .Module }}/internal/features/posts/migrations"`. Use a
plain import in `module.go` where no name clashes.

- [ ] **Step 3: wire returns a Wiring**

In each of the three `wire.go.tmpl` files:

- Change the signature to
  `func wire(engine *drel.Engine, cfg Config) (*avero.Wiring, error)`.
- Change every early error return to `return nil, err`.
- Change the last statement to
  `return &avero.Wiring{Router: r, Modules: modules}, nil`.
- Delete the `migrationSets` function.
- Delete the import of the migrations package, and delete the import of
  `io/fs` when no other line uses it. The `ssr` and `spa` shapes still use
  `io/fs` for the asset block, so keep it there.

Add this comment above the return, so a reader learns the seam:

```go
	// Wiring states what this application is. Add a dependency of your own
	// with a lifecycle in Components, and its boot check in Checks.
```

- [ ] **Step 4: main.go states three fields**

In each of the three `main.go.tmpl` files, delete the `Migrations` line:

```go
	os.Exit(avero.Serve(avero.Service[Config]{
		Args: os.Args[1:],
		Wire: wire,
		DSN:  func(c Config) string { return c.DatabaseURL },
	}))
```

- [ ] **Step 5: The acceptance test reads one list**

In each of the three `acceptance_test.go.tmpl` files, the test applies the
migrations before it calls `wire`. Reverse the order, because the migrations
now come from the module set that `wire` returns. `wire` touches no database,
so the call is safe before the migrations run.

```go
	cfg := Config{}
	cfg.Secret = avero.Secret(strings.Repeat("k", 64))
	w, err := wire(engine, cfg)
	if err != nil {
		t.Fatalf("the wiring failed: %v", err)
	}
	if _, err := engine.ApplyMigrationsFS(context.Background(), w.Modules.Migrations()...); err != nil {
		t.Fatalf("the migrations do not apply: %v", err)
	}
	handler, err := w.Router.Handler()
```

Read the real file first. Keep every other line of the helper as it stands,
and change only the order, the call to `wire` and the two reads of the result.

- [ ] **Step 6: Write the examples again**

Run: `just examples`
Then run: `go generate ./...`

- [ ] **Step 7: Prove the examples build and pass**

Run:
```bash
go run ./internal/cmd/averoexamples -check
go run ./internal/cmd/averoexamples -assets
cd examples/blog && go build ./... && go test ./... -count=1 && cd ../..
cd examples/board && go build ./... && go test ./... -count=1 && cd ../..
cd examples/orders && go build ./... && go test ./... -count=1 && cd ../..
```
Expected: PASS for each application.

- [ ] **Step 8: Prove the doctor and the routes still answer**

Run:
```bash
cd examples/blog && go run . avero:routes && DATABASE_URL=file:doctor.db go run . avero:doctor; cd ../..
```
Expected: the route table prints, and the doctor prints its table with a
database row. Delete `examples/blog/doctor.db` afterwards if it appears, and
prove `git status` is clean of it.

**Note:** read `inspect.go` for the real prefix of an inspection command
before you run this. Use the prefix that `InspectPrefix` states.

- [ ] **Step 9: Measure**

Run:
```bash
for e in blog board orders; do
  echo "== $e"
  (cd examples/$e && find . -name '*.go' ! -name '*_gen.go' ! -name 'zz_*' \
    ! -name '*_drel.go' ! -name '*_templ.go' ! -name '*_test.go' | xargs wc -l | tail -1)
done
```
Record the three numbers. They stood at 412, 390 and 398 before this branch.
Expect about 405, 383 and 391. A number above 410 for `board` means a
deletion did not land.

- [ ] **Step 10: Run the whole gate**

Run: `just verify`
Expected: every step passes.

- [ ] **Step 11: Commit**

```bash
git add scaffold/templates examples
git commit -m "Write the templates with the Wiring and with module migrations

Claude-Session: https://claude.ai/code/session_018m68TtjNW98aezz2u79Qiw"
```

---

### Task 6: The documents state the new form

**Files:**
- Modify: `docs/getting-started.md`
- Modify: `docs/configuration.md`
- Modify: `docs/agents.md`
- Modify: `docs/internal/sdd.md`
- Modify: `scaffold/templates/agents/claude-skills/add-slice/SKILL.md.tmpl`
- Modify: `CHANGELOG.md`

**Interfaces:**
- Consumes: every change of Tasks 1 to 5.
- Produces: nothing that a later task reads.

- [ ] **Step 1: Find every stale passage**

Run:
```bash
grep -rn "migrationSets\|Migrations:\|DSNEnv\|\*avero.Router, \*avero.ModuleSet\|Service\[Config\]" \
  docs scaffold/templates README.md CHANGELOG.md
```
Every hit is a passage to change. The brief's list of files may be incomplete,
so state in your report which files this grep found that the list did not
name.

- [ ] **Step 2: Change `docs/getting-started.md`**

Replace the `main.go` listing and the `wire.go` listing with the real ones
from `examples/blog`. Copy them, do not write them from memory.

Add one section after the `wire.go` listing:

```markdown
## Add a dependency of your own

Avero carries no emailer, no blob store and no cache. Build one in `wire` and
state it in the wiring.

    func wire(engine *drel.Engine, cfg Config) (*avero.Wiring, error) {
        r := avero.NewRouter()
        r.Use(avero.Stack{Secret: cfg.Secret, Engine: engine}.Middleware()...)

        mailer := smtp.New(cfg.SMTPURL)

        modules := avero.Modules(posts.New(engine, mailer))
        if err := modules.Attach(r); err != nil {
            return nil, err
        }

        return &avero.Wiring{
            Router:     r,
            Modules:    modules,
            Components: []avero.Component{mailer},
            Checks:     []avero.Check{mailer.Check()},
        }, nil
    }

Avero starts a component before the server accepts a request. It stops the
component after the last request drains. It runs a check before the process
serves, and `avero doctor` reports it.

A constructor must not dial, connect or read a file. `wire` also runs for an
inspection command with a nil engine. Build the value in the constructor.
Connect in `Start`.
```

Use the code fence style that the file already uses.

- [ ] **Step 3: Change `docs/configuration.md`**

Delete every passage about `DSNEnv`. State that `avero doctor` loads the
configuration and reads the database address through the `DSN` function of the
service.

- [ ] **Step 4: Change `docs/agents.md`**

State that a slice carries its own migrations through a `Migrations` method,
and that `wire.go` holds no list.

- [ ] **Step 5: Change `docs/internal/sdd.md`**

In section 5.2, the list of optional module interfaces already states
`MigrationModule`. Add one sentence after the list:

```markdown
A scaffolded slice implements `MigrationModule`, so `avero.Serve` reads the
migration files of the application from the module set.
```

In section 5.3, add one sentence after the numbered list:

```markdown
`wire` returns an `avero.Wiring`. It carries the router, the module set, the
components of the application and its boot checks. `avero.Serve` reads all
four.
```

- [ ] **Step 6: Change the add-slice skill**

In `scaffold/templates/agents/claude-skills/add-slice/SKILL.md.tmpl`, add a
step: the module states a `Migrations` method that returns the embedded file
system of the slice. State that `wire.go` needs no change for a migration.

- [ ] **Step 7: Add the changelog entry**

Add this block under the unreleased heading of `CHANGELOG.md`, in the style
that the file already uses:

```markdown
### Added

- `avero.Wiring` carries the router, the module set, the components and the
  boot checks of an application. `wire` returns it.
- An application adds a dependency with a lifecycle and a boot check, and
  keeps the lean `main.go`.
- `avero doctor` loads the configuration and reads the database address from
  it.

### Changed

- `Service.Wire` returns one `*avero.Wiring` and an error.
- A scaffolded slice states its own migrations through a `Migrations` method.
  `wire.go` holds no list.

### Removed

- `Service.Migrations`. The module set states the migration files.
- `Service.Components`. `Wiring.Components` replaces it and reaches the values
  that `wire` built.
- `Service.DSNEnv`. The doctor reads the address from the configuration.
```

- [ ] **Step 8: Write the examples again and run the gate**

Run:
```bash
just examples
just verify
```
Expected: every step passes.

- [ ] **Step 9: Commit**

```bash
git add docs scaffold/templates CHANGELOG.md examples
git commit -m "State the Wiring and the module migrations in the documents

Claude-Session: https://claude.ai/code/session_018m68TtjNW98aezz2u79Qiw"
```

---

## Verification

The work is complete when each of these holds:

- [ ] `just verify` passes with no failed step.
- [ ] `examples/blog/main.go` states three fields and no `Migrations`.
- [ ] No `wire.go` in `examples/` holds a `migrationSets` function.
- [ ] Each slice module in `examples/` holds a `Migrations` method.
- [ ] `avero routes` runs on the blog example with no `DATABASE_URL` and no `AVERO_SECRET`.
- [ ] `avero doctor` on the blog example reports a database row that came from the configuration.
- [ ] A test proves a component from the wiring starts and stops in the right order.
- [ ] A test proves a check from the wiring stops a bad boot.
- [ ] `grep -rn "DSNEnv\|Service.Migrations\|Service.Components" . --include='*.go' --include='*.md' --include='*.tmpl'` returns no hit outside the changelog.
