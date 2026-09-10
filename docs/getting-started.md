# Getting started

This page writes one application and runs it.

## Before you start

You need Go 1.26 or a later version. You need no Node.js. SQLite needs no
server, so the first application needs no database of its own.

## Write the application

```
go install github.com/alternayte/avero/cmd/avero@latest

avero new blog
cd blog
```

`avero new` writes the application, reads its dependencies, and writes every
generated file: the templ pages of the ssr shape, the binding of each input
type and the implementation of each client. The shape is `ssr` by default.
Write `--shape spa` for a React front end, or `--shape api` for a JSON
service.

## Run it

```
cp .env.example .env
avero migrate up
avero dev
```

The application answers on http://localhost:8080. `avero dev` watches the tree.
A change to a stylesheet swaps the link element with no reload. A change to a
Go file rebuilds the binary and morphs the page, so the page keeps its scroll
position.

## Read the application

`main.go` calls `avero.Serve`, which runs the start sequence. It answers
`avero routes` and `avero doctor`, loads the configuration, and opens the
database. It builds the router and registers the boot checks and the
migrator, then serves until a signal. An application with an unusual start
calls `avero.Load`, `avero.New` and `Run` itself.

```go
func main() {
	os.Exit(avero.Serve(avero.Service[Config]{
		Args: os.Args[1:],
		Wire: wire,
		DSN:  func(c Config) string { return c.DatabaseURL },
	}))
}
```

`wire` builds the wiring of the application: the router, the module set, the
components and the boot checks.

```go
func wire(engine *drel.Engine, cfg Config) (*avero.Wiring, error) {
	r := avero.NewRouter(avero.WithForm(func(c *avero.Ctx, f *avero.Fields) avero.ViewComponent {
		// The form failed validation. The page renders again with the old
		// input and the field errors. See S10.
		return ui.Page("New post", ui.NewPostForm())
	}))

	// An inspection command carries a zero configuration. The router only
	// records the middleware there, so a fixed value keeps the chain of an
	// inspection identical to the chain of a run.
	secret := cfg.Secret
	if secret == "" {
		secret = avero.Secret(strings.Repeat("0", 64))
	}
	// Stack states the order of the chain one time. A nil engine leaves the
	// transaction out. Add the session and the authentication of auth-all in
	// the Session and Auth fields, and Stack puts them in the correct place.
	r.Use(avero.Stack{Secret: secret, Engine: engine}.Middleware()...)

	// MountAssets reads the manifest and serves the built files at /assets/.
	// The binary carries them, so the server needs no directory beside it.
	manifest, err := avero.MountAssets(r, dist, "assets/dist")
	if err != nil {
		return nil, err
	}
	ui.SetManifest(manifest)

	modules := avero.Modules(posts.New(engine))
	if err := modules.Attach(r); err != nil {
		return nil, err
	}
	// Wiring states what this application is. Add a dependency of your own
	// with a lifecycle in Components, and its boot check in Checks.
	return &avero.Wiring{Router: r, Modules: modules}, nil
}
```

```
blog/
├── main.go                   the composition root
├── config.go                 every variable that the application reads
├── wire.go                   the router, the middleware and the modules
├── avero.json                the shape and the asset pipeline
├── internal/features/posts/  one feature: routes, handlers, inputs, store
│   ├── model/                the rows that drel reads
│   └── migrations/           the SQL of this feature. drel writes it.
├── internal/ui/              the templ pages. They import no feature package.
├── drel.yaml                 the model packages and the migrations of drel
├── assets/                   the source of the stylesheet and of the script
├── AGENTS.md                 the rules that an agent reads
└── .claude/skills/           one skill for each task that repeats
```

## Add a dependency of your own

Avero carries no emailer, no blob store and no cache. Build one in `wire` and
state it in the wiring.

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

Avero starts a component before the server accepts a request. It stops the
component after the last request drains. It runs a check before the process
serves, and `avero doctor` reports it.

Serve runs every check before it starts any component. A check that depends
on a component, such as the mailer above, must open its own connection and
close it. It must not assume the connection that `Start` of that component
opened. `avero doctor` runs the checks and starts no component.

A constructor must not dial, connect or read a file. `wire` also runs for an
inspection command with a nil engine. Build the value in the constructor.
Connect in `Start`.

## Add a feature

```
avero slice comment
avero migrate up
avero verify
```

`avero slice` writes the module, the model, the input types, the handlers, the
store and the test. It adds the slice to `drel.yaml`, writes the first
migration of the table, and registers the module in `wire.go`.

## Read and write the database

A store reads and writes with the typed API of drel. It writes no SQL string.

```go
repo, err := avero.Repo(ctx, model.PostMeta)
if err != nil {
	return nil, err
}
return repo.AsNoTracking().
	Where(model.Posts.Title.Contains(search)).
	OrderBy(model.Posts.ID.Desc()).
	All(ctx)
```

`avero.Repo` returns the repository of the transaction of the request. The
transaction middleware opens that transaction, so a handler always holds one.

`model.Posts` is generated. A column that no model names does not compile, so a
name fault appears before the process starts. See design rule 8.

A write stages the change on the transaction of the request:

```go
post := model.NewPost(title, body)
repo.Add(post)
```

The key is a UUID of version 7. drel stamps it at `Add`, so the answer of the
handler carries the identifier before the commit. Call `avero.Save(ctx)` to
flush the staged change, so a later read of the same request sees it. The
transaction still commits at the end of the request.

## Change a table

Add a field to the model, then write the migration:

```
avero generate
avero migrate new add_a_column
avero migrate up
```

drel compares the model with its snapshot and writes the two SQL files. Read
them before you apply them.

## Prove the work

```
avero verify
```

The gate runs gofmt, go vet, the generated file check, the tests and the build.
`avero verify --json` writes one record for each step, which an agent reads.

## Read the state of the application

```
avero routes      every route with its middleware
avero modules     the contribution of each module
avero schema      the models that the modules describe
avero doctor      the configuration, the database and the migrations
```

Each command takes `--json`, and each JSON answer carries the address of its
schema.
