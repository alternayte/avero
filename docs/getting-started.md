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

`avero new` writes the application, the generated files and the starter assets.
The shape is `ssr` by default. Write `--shape spa` or `--shape api` for another
shape.

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

```
blog/
├── main.go                   the composition root
├── config.go                 every variable that the application reads
├── wire.go                   the router, the middleware and the modules
├── avero.json                the shape and the asset pipeline
├── internal/features/posts/  one feature: routes, handlers, inputs, store
├── internal/ui/              the views. They import no feature package.
├── migrations/               the SQL files
├── assets/                   the source of the stylesheet and of the script
├── AGENTS.md                 the rules that an agent reads
└── .avero/skills/            one file for each task that repeats
```

## Add a feature

```
avero slice comment
avero migrate up
avero verify
```

`avero slice` writes the module, the input types, the handlers, the store, the
test and the migration. It registers the module in `wire.go`.

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
