# GraphQL

Avero adds no GraphQL code. This page states the pattern that works with the
module contract.

## The pattern

- Each slice keeps its own `.graphqls` file, beside its handlers.
- `gqlgen.yml` globs `internal/features/*/*.graphqls`.
- Each slice exports a `Resolver` type.
- The root resolver embeds them. Go method promotion performs the merge.
- Mount the handler on the router:

```go
r.Mount("/graphql", srv)
```

A mounted handler writes to the ResponseWriter itself, so no transaction
surrounds it. Open the transaction in the resolver with `drel.Engine.WithTx`.

## The known limit

gqlgen emits one executable schema. The generated code is therefore central,
although the source is sliced. A change to one slice regenerates the whole
schema.

## The gate

Add the generator to the gate of your application:

```
//go:generate go run github.com/99designs/gqlgen generate
```

`avero verify` runs `go vet`, the tests and the build, so a stale resolver
stops the gate.
