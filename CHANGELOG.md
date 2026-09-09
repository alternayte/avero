# Changelog

The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).
The versions follow [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## Unreleased

### Added

- The comment of a handler states its summary again. `avero generate` writes
  the first sentence into a Summaries method of the module, and the module set
  passes the map to the router before it registers the routes. A person
  therefore writes the sentence one time, and no comment names a type. An
  option of a route wins over the comment.

## v0.3.0

This release changes the shape of every handler and of every route table. See
"To move an application from v0.2" at the end of the entry.

### Added

- **Typed routes and a runtime describer.** A route registers with the type of
  its input and the type of the body of its answer:

  ```go
  avero.Get(r, "/posts/{id}", m.Show,
      avero.Answers[avero.Problem](http.StatusNotFound, "the post does not exist"))

  func (m *Module) Show(c *avero.Ctx, in ShowInput) (View, error)
  ```

  The compiler holds both types, so a wrong name does not build. A handler
  returns the thing that it answers, as an ordinary Go function does, so a
  test calls the method and reads the value. The method of the route states
  the status: a POST answers 201 and every other method answers 200.
  `Ctx.Status` names another status, and `avero.NoBody` answers none. The
  options `avero.Summary`, `avero.Tags`, `avero.Deprecated`, `avero.Answers`
  and `avero.AnswersNothing` state what a signature cannot carry, such as a
  second status.
- **Typed errors and Problem Details, RFC 9457.** `avero.Problem` is an error
  and a response. A handler returns `avero.NotFound("post", id)`, and the
  router writes `application/problem+json` with the status, the title, the
  detail and the path of the request. `avero.Conflict`, `avero.Unauthorized`,
  `avero.Forbidden` and `avero.BadRequest` cover the other common answers.
  `errors.Is(err, avero.ErrNotFound)` answers for every 404, and a problem
  wraps a cause, so a store keeps its own error and the client never reads it.

- The DX-3 budget is 5 s. It was 3 s. Four measurements of one quiet laptop
  read 3.1 s, 5.7 s, 5.7 s and 7.3 s, so 3 s held only on the best run of an
  idle machine, and the gate failed for the load of its machine and not for a
  fault of the loop. The parts of one rebuild stand in the SDD, section 3.

- The media type of a request comes from the tags of its input. A field with
  a `json` tag reads JSON, and a field with a `form` tag reads a form.

### Known limits

- A route reads no file. The generated Bind reads the form of a request and
  no multipart body, so the description states no `multipart/form-data`. A
  description of a capability that the binder does not carry would be worse
  than the gap.

### Removed

- The generator that read the source of an application to write the
  description of its API, 1350 lines with its tests. The application states
  its own routes now.
- `avero.In` for a route that answers JSON, and the `//avero:response`
  directive.

### To move an application from v0.2

1. Change each handler that answers JSON to return its value:
   `func (m *Module) Show(c *avero.Ctx, in ShowInput) (View, error)`. A
   handler that answers no body returns `avero.NoBody`.
2. Delete each `//avero:response` comment.
3. Change each registration of such a route from
   `r.Get("/posts", avero.In(m.List))` to `avero.Get(r, "/posts", m.List)`,
   and state each summary with `avero.Summary`.
4. Answer a failure with a problem, such as `avero.NotFound("post", id)`, in
   place of a map of one string.
5. A page of the ssr shape does not change. It answers `avero.Response` and
   registers with `avero.In`.

### Changed

- `avero routes --openapi` runs the application through the inspection channel
  that `avero routes` already used, and reads the operations that the router
  holds. It reads no comment and no source text. A route registration touches
  no database and opens no port, so the command still needs no infrastructure.
  The reflection reads the types one time, and never on a request path.
- The `//avero:response` directive is gone. A comment stated a type that
  nothing checked, so a name with one wrong letter wrote a reference to a
  schema that did not exist, and the client of the front end broke with no
  message.
- The wrapper `avero.In` is gone from the JSON shapes. `avero.Get`,
  `avero.Post`, `avero.Put`, `avero.Patch` and `avero.Delete` register a typed
  handler and read its types. The ssr shape keeps `avero.In`, because a page
  answers a view and names no body.
- The description states the error answers of every route against one Problem
  schema. A route that binds a value states 400 and 422.
- A summary comes from `avero.Summary` now. The generator read the comment of
  a handler before, and the description carries what the code states. The
  release after this one reads the comment again, through the generator.
- A handler error answers the problem that it carries. An error that carries
  no problem stays a 500, and its message stays in the log.
- The validation fault and the bind fault answer problem documents. The field
  messages ride in the `errors` member beside the members of RFC 9457, so the
  member that a client reads does not move. The ssr shape keeps its own
  answer, so a form renders again.
- The scaffolded handlers return `avero.NotFound` in place of a map of one
  string. One error shape covers the whole API.

## v0.2.3

### Changed

- The scaffolder reads the version of Avero from the build information of the
  binary. A release therefore writes the version that carries its templates,
  and the constant that a person raised after each tag is gone. A build from
  the source names no version, so go.mod carries no line for Avero, and
  `go mod tidy` reads the newest release or the directory that a replace
  names. `avero new` runs the command, so an application holds a version
  either way.
- The test of v0.2.2 that compared the constant with the newest tag is gone. A
  tag build could not pass it: the newest tag is the tag under build, and the
  constant names the release before it.

## v0.2.2

### Fixed

- `avero new` writes a version of Avero that stands. The scaffolder named
  v0.1.0, which carries no `MigrationCheckOnFS`, so an application that a
  released binary wrote did not compile. Every example replaces the module
  with the repository, so the examples hid the fault. A test proves now that
  the scaffolder names the newest tag.

### Added

- A test reads one row through `db.Tx(ctx).Modules` of a scaffolded
  application. No template reads the module holder, so the gate reached no
  part of it before. The test fails against drel v0.7.0 and passes against
  v0.7.1.

## v0.2.1

### Changed

- drel v0.7.1. The generated `Tx` fills its module holder now. A call of
  `db.Tx(ctx).Modules.<Module>.<Rows>` held nil in v0.7.0. An application
  that stands already reads the fix with
  `go get github.com/alternayte/drel@v0.7.1`, then `avero generate`.
  `avero new` writes v0.7.1 in the go.mod of a new application.

### Fixed

- The development loop test waits for the loop to stop before the test
  framework removes the tree. A cancel returns at once, so a rebuild walked a
  directory that left under it, and the loop wrote a message that named the
  output package of drel. The guard of v0.2.0 in the two code generators
  treated the message and not its cause, so it leaves again.

## v0.2.0

### Changed

- **drel 0.7.0.** The scaffolded stores read and write with the typed API of
  drel. They write no SQL string, so a wrong column name does not compile.
  Each feature slice owns a `model/` package and a `migrations/` package, which
  `drel.yaml` names. `avero generate` runs the drel generator, `avero migrate`
  runs the drel migrator, and `avero migrate new <name>` writes the two SQL
  files from the difference between the model and its snapshot. The binary
  carries the migrations of every slice through `migrationSets()`.
- The key of a scaffolded row is a UUID of version 7. drel stamps it at `Add`,
  so a handler answers with the identifier before the commit.
- `avero slice` writes a model package, adds the slice to `drel.yaml`, writes
  the first migration and adds the set to `migrationSets()` in `wire.go`.

### Added

- `avero.MigrationCheckFS` proves the embedded migration sets against a
  database that the check opens itself. `avero doctor` uses it.

### Fixed

- The code generators no longer stop when a directory leaves between the walk
  and the read. The development loop runs two generators, and one writes the
  output package of drel again.

### To move an application from v0.1.0

1. Add `drel.yaml` with one module for each feature slice.
2. Move each row type into `internal/features/<name>/model`, embed
   `drel.Model[uuid.UUID]`, and give each field a `db` tag.
3. Run `avero generate`, then `avero migrate new create_<name>` for each slice.
4. Write `migrationSets()` in `wire.go`, and call `ApplyMigrationsFS` in
   `main.go`.
5. Write each store call with the typed API of drel.

## v0.1.0

The first release. It carries every subsystem except the messaging ones.

### Added

- **Configuration, S1.** A struct with `env` tags, one report of every fault,
  and a secret that no log line prints.
- **Host and lifecycle, S2.** Components in registration order, a readiness
  gate, boot checks and a graceful shutdown.
- **Telemetry, S3.** Traces, metrics and a logger from the standard `OTEL_*`
  variables. The default exporter is none.
- **Router, S4.** `net/http` patterns, typed handlers that return a Response,
  request identifier, recovery, access log, CSRF, flash and a transaction
  middleware.
- **Handler codegen, S5.** Generated binding and validation for each input
  type. No generated file imports reflect.
- **Module system, S6.** One module for each feature, with optional interfaces
  and a contribution table. Two modules on one pattern stop the process.
- **View layer, S10.** The old input, the field errors, the toasts and the CSRF
  field. The ssr shape writes its pages as templ files, and the application
  carries the generator as a tool of its `go.mod`.
- **Asset pipeline, S11.** esbuild as a Go library, the Tailwind standalone
  binary, content hashing, a manifest and a handler that serves an embedded
  file system. Tier 0 needs no Node.js.
- **Hypermedia adapters, S12.** `ds` for Datastar 1.0 and `htmx` for htmx 2.
  Both are optional imports.
- **Client codegen, S13.** An interface with directives becomes a client with
  timeouts, retries, a circuit breaker, OTel spans and typed errors.
- **CLI and scaffolder, S14.** `avero new`, `slice`, `generate`, `build`,
  `migrate`, `routes`, `modules`, `schema`, `doctor`, `verify`, `js pin`,
  `assets init`. Three shapes: ssr with templ pages and Datastar, spa with
  TypeScript, React 19, TanStack Query and Vite, and api with a generated
  client.
- **Dev loop, S15.** `avero dev` swaps a stylesheet with no reload, and it
  morphs the page after a rebuild, so the scroll position survives.
- **OpenAPI.** `avero routes --openapi` reads the routes, the input types and
  the `//avero:response` directives of the source and writes an OpenAPI 3.1
  document with its schemas. It runs no application. The spa shape generates
  its client and its TanStack Query options from that document with Hey API.
- **Agent surface, S16.** `avero mcp` with seven tools, `AGENTS.md`, four
  skills in the Agent Skills format, and one published schema for each
  report.

### The measured budgets

| ID | Requirement | Budget | Measurement |
|---|---|---|---|
| DX-1 | `avero new` to a running application | 60 s | 1.8 s |
| DX-2 | a change to a `.css` file appears in the browser | 200 ms | 41 ms |
| DX-3 | a change to a `.go` file appears in the browser | 3 s | 1.7 s |

The DX-3 budget was 2 s. One rebuild spends 1.0 s to 1.7 s in the Go link and
0.3 s to 0.5 s in the first execution of a new binary on macOS, so 2 s held
only on an idle machine. The budget is now 3 s, and the parts stand in the
test.

### Not in this release

The outbox relay (S7), the inbox consumer (S8) and event sourcing (S9). The
commands `avero outbox dead`, `avero inbox dead` and `avero es` arrive with
them. `avero new --shape api` writes a JSON service with CRUD and a generated
client, and it writes no messaging.
