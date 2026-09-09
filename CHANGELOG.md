# Changelog

The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).
The versions follow [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

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
  field. A templ component satisfies the component interface.
- **Asset pipeline, S11.** esbuild as a Go library, the Tailwind standalone
  binary, content hashing, a manifest and a handler that serves an embedded
  file system. Tier 0 needs no Node.js.
- **Hypermedia adapters, S12.** `ds` for Datastar 1.0 and `htmx` for htmx 2.
  Both are optional imports.
- **Client codegen, S13.** An interface with directives becomes a client with
  timeouts, retries, a circuit breaker, OTel spans and typed errors.
- **CLI and scaffolder, S14.** `avero new`, `slice`, `generate`, `build`,
  `migrate`, `routes`, `modules`, `schema`, `doctor`, `verify`, `js pin`,
  `assets init`. Three shapes: ssr, spa and api.
- **Dev loop, S15.** `avero dev` swaps a stylesheet with no reload, and it
  morphs the page after a rebuild, so the scroll position survives.
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
