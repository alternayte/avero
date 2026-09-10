# Wiring extensibility — design

Date: 2026-09-10. Written in ASD-STE100.

Follows `2026-09-10-lean-user-code-design.md`. That work added `avero.Serve`.
This work repairs a gap that `avero.Serve` opened.

## 1. Problem

`avero.Serve` runs the start sequence of an application. It closed a door that
the hand-written `main.go` held open.

A user adds a dependency that Avero does not carry: an emailer, a blob store, a
cache, a search client. Two halves of that work must succeed. Today one fails.

**Injection works.** `wire` is user code and it receives the configuration. The
user builds the client there and passes it to the module constructor.

```go
mailer := smtp.New(cfg.SMTPURL)
modules := avero.Modules(posts.New(engine, mailer))
```

**Lifecycle fails.** A dependency with a `Start` and a `Stop` must be an
`avero.Component`. Neither seam of `Service` reaches the value that `wire`
built.

- `Components func(set *ModuleSet) []Component` receives the module set only.
  The mailer is not in it.
- `Options []Option` is stated in `main.go`, and `main.go` holds no
  configuration. `Serve` loads it.

Before `avero.Serve`, `main.go` called `avero.Load` itself. A user built the
mailer between the load and the call to `wire`, and passed it to both.

A boot check for a user dependency fails for the same reason. DX-8 says a fault
appears before the process starts. A user who adds a blob store cannot state a
check that proves the bucket answers.

The escape hatch is real but bad: `avero.New`, `avero.Load` and `avero.Inspect`
stay public, so the user writes the long `main.go` again. That says the lean
form works until the application grows.

## 2. Goal

A user adds a dependency with a lifecycle and a boot check, and keeps the lean
`main.go`.

## 3. Non-goals

- `Serve` does not start the jobs, the message handlers or the projections of
  the module set. The outbox and the inbox are their own work. See S7 and S8.
- No service container. Section 1.1 of the SDD forbids it.
- No change to how a module states its migrations.

## 4. The design

### 4.1 The Wiring type

`wire` returns one value in place of three.

```go
// Wiring is what wire builds. Serve reads it.
type Wiring struct {
	// Router holds the routes of the application. It is required.
	Router *Router
	// Modules holds the module set. It is required.
	Modules *ModuleSet
	// Components are the parts of the application that hold a lifecycle.
	// Serve starts them in this order and stops them in reverse order.
	Components []Component
	// Checks are the boot checks of the application. Serve runs them before
	// the process serves, and `avero doctor` reports them.
	Checks []Check
}
```

`Service.Wire` becomes:

```go
Wire func(engine *drel.Engine, cfg C) (*Wiring, error)
```

`Service.Components` goes away. `Wiring.Components` replaces it and reaches the
values that `wire` built. `Service.Options` stays as the escape hatch.

`Serve` states a fault when `Wiring` is nil, when `Router` is nil or when
`Modules` is nil. The fault names the repair.

The user code then reads:

```go
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
```

### 4.2 Order

`Serve` registers the components after the boot checks and before the handler.
The host starts a component in registration order and stops it in reverse
order. A user dependency therefore starts before the server accepts a request.
It stops after the last request drains.

`Serve` appends `Wiring.Checks` after the checks that it builds, so the secret,
the database and the migrations are proved first.

### 4.3 A constructor must not connect

`wire` runs during an inspection with a nil engine and a zero configuration.
A constructor must not dial, connect or read a file. Build the value in the
constructor. Connect in `Start`.

This rule holds for the router today. `Wiring.Components` makes it load-bearing
for a user dependency, so the doc comment of the field must state it.

### 4.4 The doctor

`Serve` reads the database address with `os.Getenv` in the inspect path today,
because that path holds no configuration. An application that composes its
address, or reads it from a file or a secret store, gets an `avero doctor` that
skips the database check and reports success.

`Inspect` already loads the configuration for the `doctor` command. It calls
`config.LoadFrom[T]`. So `Serve` splits the inspect path in two.

| Command | Configuration | Engine | Checks |
|---|---|---|---|
| `doctor` | loaded | opened when `DSN` is not nil | the database check, the migration check and `Wiring.Checks` |
| `routes`, `modules`, `schema`, `openapi` | zero | nil | none |

`avero routes` therefore still runs on a machine with no database and no
configuration. `avero doctor` reports the checks of the application, including
the checks of a user dependency.

A configuration that does not load does not stop the doctor. `Inspect` reports
the fault itself. `Serve` passes what it has.

The `DSNEnv` field of `Service` is then unnecessary and it goes away. It exists
only to name the variable that the old inspect path read.

## 5. Tests

1. A component that `wire` builds starts and stops with the application.
2. A check that `Wiring` carries stops a bad boot with the exit code 1, and the
   error names the repair.
3. `avero doctor` reports a check that `Wiring` carries.
4. `avero routes` runs with no database and no configuration.
5. A nil `Wiring`, a nil `Router` and a nil `Modules` each state a fault that
   names the repair.
6. A component stops in reverse order.

## 6. Constraints

- The examples are generated from `scaffold/templates`. Change a template, then
  run `just examples`.
- `just verify` must pass.
- Every new export carries a doc comment in ASD-STE100.
- No released API changes. `Service` and `Wire` arrived on the `lean-user-code`
  branch and no release carries them.
- The work starts from the `lean-user-code` branch, because `Serve` lives there.

## 7. What this does not repair

`Service.Migrations` still states the migration sets that a module could state
through `MigrationModule`. No module implements that interface today, so a
fallback would be dead code. It belongs in its own change.
