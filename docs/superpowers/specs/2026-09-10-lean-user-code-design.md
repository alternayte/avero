# Lean user code — design

Date: 2026-09-10. Written in ASD-STE100.

## 1. Problem

A person who writes an Avero application writes about 532 lines of Go for the
blog reference application. About 170 of those lines are ceremony. Ceremony is
code that every application repeats and that states nothing about the
application.

Two measurements prove it:

- `examples/blog/main.go`, `examples/orders/main.go` and
  `examples/board/main.go` differ by one comment line. Three application
  shapes, one composition root.
- `examples/blog/.../store.go` and `examples/orders/.../store.go` differ by one
  comment line.

| File | Lines | Ceremony |
|---|---|---|
| `main.go` | 93 | 90 |
| `store.go` | 128 | 25 |
| `wire.go` | 70 | 12 |
| `module.go` | 55 | 18 |
| other | 186 | 0 |

## 2. Goal

Remove about 145 ceremony lines from each application, and about 43 from each
new feature slice. Keep every design rule of the SDD.

## 3. Non-goals

- No attribute route and no route discovery. Design rule 2 forbids runtime
  reflection on a request path.
- No service container. Section 1.1 of the SDD forbids it.
- No change to `input.go`, to `handlers.go` or to `model/post.go`. That code is
  the intent of the application.

## 4. The four changes

### 4.1 `avero.Serve` replaces the body of `main.go`

Design rule 3 says that `main.go` supplies the dependencies. Today `wire.go`
supplies them, and `main.go` holds only the sequence that section 5.3 of the
SDD already fixes. `avero.Serve` names that sequence one time.

```go
func main() {
	os.Exit(avero.Serve(avero.Service[Config]{
		Args:       os.Args[1:],
		Wire:       wire,
		DSN:        func(c Config) string { return c.DatabaseURL },
		Migrations: migrationSets,
	}))
}
```

`avero.New`, `avero.Load`, `avero.Inspect` and `avero.Exit` stay public. An
application with an unusual start writes the long form.

### 4.2 `Models()` comes from the generator

The `Describe` method of a module restates the model struct by hand. AN-3 says
that the application describes itself. A description that a person retypes can
disagree with the model, and no gate catches it.

`avero generate` reads the sibling `model` package and writes a `Models`
method. The module system merges it into the description. The hand-written
`Describe` method goes away.

### 4.3 `avero.Repo` and `avero.Save` replace the store helpers

Every store repeats the same two unexported methods. Both read the transaction
from the context and return the same repair sentence.

```go
repo, err := avero.Repo(ctx, model.PostMeta)
```

The store keeps its engine field and its constructor, so design rule 3 holds.

### 4.4 `avero.MountAssets` replaces three blocks of `wire.go`

`LoadManifest`, `fs.Sub` and `Mount` always appear together, and each one can
fail. One call does the three.

## 5. Constraints

- The examples are generated from `scaffold/templates`. Change a template, then
  run `just examples`. Never edit an example by hand.
- `just verify` must pass. See the definition of done in AGENTS.md.
- Every new export carries a doc comment in ASD-STE100.
- DX-5 holds: a cold `go build` of a scaffolded SSR application takes 15 s or
  less.
