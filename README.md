# Avero

Avero is an application host for Go services. It owns the composition root, the
lifecycle, the configuration, the telemetry and the routing. It does not own
the semantics of your application.

Avero is not a web framework. It writes plain Go that you can read, change and
delete.

## Start

```
go install github.com/alternayte/avero/cmd/avero@latest

avero new blog
cd blog
cp .env.example .env
avero migrate up
avero dev
```

The application answers on http://localhost:8080. A change to a `.css` file
reaches the browser in about 40 milliseconds. A change to a `.go` file reaches
it in under two seconds.

You need Go, and a database when you leave SQLite. You need no Node.js.

## What you get

- One command writes an application that builds, runs and passes its tests.
- Typed handlers with generated binding and validation. No reflection runs on
  a request path.
- A module contract, so one directory holds one feature.
- Server rendered pages with forms, CSRF, flash messages and validation.
- An asset pipeline with esbuild and the Tailwind standalone binary.
- Datastar and htmx, as optional imports.
- An HTTP client generator with timeouts, retries, a circuit breaker and OTel
  spans.
- A development loop that swaps a stylesheet with no reload and morphs the page
  after a rebuild.
- An MCP server, so an agent reads the routes, the modules and the models, and
  writes a new slice.

## The commands

```
avero new <name>      write a new application
avero slice <name>    write a new feature slice
avero dev             run the application and rebuild it on each change
avero generate        write the generated files
avero build           generate, build the assets and compile the binary
avero migrate         write and apply the migrations
avero routes          print the routes
avero modules         print the contribution of each module
avero schema          print the models
avero doctor          prove the configuration and the database
avero verify          run the gate
avero mcp             serve the agent surface over stdio
avero js pin <pkg>    fetch a bundled module into the vendor directory
avero assets init     write the package.json of tier 1
```

## The shapes

| Shape | Application | Exercises |
|---|---|---|
| `ssr` | a blog with posts | the router, the views, the forms and the assets |
| `spa` | a task board | the JSON routes and the embedded front end |
| `api` | an order service | the JSON service and the generated client |

`examples/` holds one application of each shape. The gate writes them again and
compares them, so they never drift from `avero new`.

An example ignores its output directory, as every application does, so a clone
holds no built asset. Write the starter assets one time:

```
go run ./internal/cmd/averoexamples -assets
cd examples/blog && go run .
```

## The state of the subsystems

| ID | Subsystem | State |
|---|---|---|
| S1 | Configuration | ready |
| S2 | Host and lifecycle | ready |
| S3 | Telemetry | ready |
| S4 | Router and request context | ready |
| S5 | Handler codegen | ready |
| S6 | Module system | ready |
| S7 | Outbox relay | planned |
| S8 | Inbox consumer | planned |
| S9 | Event sourcing | planned |
| S10 | View layer | ready |
| S11 | Asset pipeline | ready |
| S12 | Hypermedia adapters | ready |
| S13 | Client codegen | ready |
| S14 | CLI and scaffolder | ready |
| S15 | Dev loop | ready |
| S16 | Agent surface | ready |

An application that publishes a message with an exactly-once effect waits for
S7, S8 and S9. Every other shape works today.

## The documents

- [Getting started](docs/getting-started.md)
- [Configuration](docs/configuration.md)
- [Routing and handlers](docs/routing.md)
- [Views and forms](docs/views.md)
- [Assets](docs/assets.md)
- [HTTP clients](docs/clients.md)
- [The agent surface](docs/agents.md)
- [GraphQL](docs/graphql.md)
- [The changelog](CHANGELOG.md)

## The gate

```
just verify
```

The gate runs gofmt, go vet, golangci-lint, the tests with the race detector,
the integration tests, the regeneration check, the three reference
applications, and the development experience budgets.

## Licence

MIT. See [LICENSE](LICENSE).
