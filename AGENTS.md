# AGENTS.md

Avero is an application host for Go services. It owns the composition root, the
lifecycle, the configuration, the telemetry, and the transactional plumbing for
messaging. It is not a web framework.

- Module path: `github.com/alternayte/avero`
- Binary and CLI: `avero`
- Specification: `docs/internal/sdd.md`. This file is local only. Read it before
  you change a subsystem. The SDD wins over this file.

## Commands

```
just verify          # the full gate. See "Definition of done".
go build ./...
go test ./... -race -count=1
go test ./... -race -tags=integration     # needs PostgreSQL and RabbitMQ
gofmt -l .
go vet ./...
golangci-lint run
```

Run one test:

```
go test ./router -race -run TestName
```

## Repository layout

One directory for each subsystem. `cmd/avero` holds the CLI. `config`, `host`,
`telemetry`, `router`, `codegen`, `module`, `outbox`, `inbox`, `es`, `view`,
`assets`, `ds`, `htmx` and `mcp` hold the subsystems. `scaffold/templates` holds
the project templates. `examples` holds the three reference applications.

## Design rules

1. No global mutable state. No facades. No package-level singletons.
2. No runtime reflection on a request path. Generate the code instead.
3. Dependencies are struct fields. The constructor takes them. `main.go` supplies
   them.
4. Generated code is plain Go. A person can read it, edit it, and delete it.
5. Avero owns lifecycle. Avero does not own semantics.
6. Explicit beats short. Do not add magic to save a line.
7. Slices do not import slices.
8. Move a fault to compile time when you can.

## Error rules

- An error names a path, a line and a column in a file that the user wrote.
- An error never points into generated code or into Avero.
- An error carries one sentence that states the repair.
- A fault appears before the process starts. `avero doctor` runs at boot.

## Test rules

- Write the test before the code that it judges.
- An acceptance test drives real HTTP against a real database and a real broker.
  Do not use mocks at that level.
- Never relax a test to make the gate pass. Never skip a test.
- Never edit a generated file by hand. Change the generator.

## Definition of done

A subsystem is complete when `just verify` passes. The steps are `gofmt`,
`go vet`, `golangci-lint`, unit tests with `-race`, integration tests, a
regeneration check with `git diff --exit-code`, the three reference applications,
and the DX budgets in section 3 of the SDD.

## Writing standard

Write every document, comment and commit message in ASD-STE100 Simplified
Technical English. Use the active voice. Use one word for one meaning. Do not
use contractions.

## Stop and ask

Work without a question unless one of these is true:

1. The change alters a public API that the SDD states.
2. The change adds a third-party dependency that the SDD does not name.
3. Two requirements in the SDD conflict.
4. A correctness property cannot hold as specified. Ordering, exactly-once effect
   and transaction boundaries are correctness properties.
5. The work needs a database table that the SDD does not describe.
6. A test that proves a stated property cannot be written.
7. A development experience budget cannot be met.
