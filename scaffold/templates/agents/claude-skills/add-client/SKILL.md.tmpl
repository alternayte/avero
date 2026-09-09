---
name: add-client
description: Use when this Avero application must call another HTTP service, or when the task names a third-party API, a webhook target or an upstream endpoint. It writes an interface with directives, and the generator writes the implementation with timeouts, retries and typed errors.
---

# Add an HTTP client

A client is an interface with comment directives. `avero generate` writes the
implementation beside it. The runtime carries the timeout, the retry with
backoff, the circuit breaker and one OTel span for each call.

## Steps

1. Write the package `internal/clients/<service>/<service>.go`.

2. Write the directive above the interface:

   ```go
   //avero:client base="https://api.example.com" auth="bearer" timeout="5s" attempts="3"
   type Payments interface {
       //avero:GET /charges/{id}
       GetCharge(ctx context.Context, id string) (Charge, error)

       //avero:POST /charges
       CreateCharge(ctx context.Context, body NewCharge) (*Charge, error)
   }
   ```

3. Give each method a directive with the HTTP method and the path. A path
   parameter in braces binds to the method parameter of the same name.

4. Give a struct `query` tags or `header` tags for the query and the headers.
   Any other struct becomes the JSON body. The results are `(T, error)` or
   `error`.

5. Run `avero generate`.

6. Write the test against `httptest`. Pass `client.WithBaseURL(srv.URL)` to the
   generated constructor.

7. Supply the credential in `main.go` with `client.WithToken`. No secret
   reaches the source.

## Rules

- A status outside the two hundreds becomes a `client.StatusError`, which keeps
  the response. Read it with `errors.As`.
- Only an idempotent method retries. Write `idempotent="true"` on a POST that
  is safe to repeat.

## The command that proves the work

```
go test ./internal/clients/... -race
```
