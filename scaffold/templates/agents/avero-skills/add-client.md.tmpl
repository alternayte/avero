# Add an HTTP client

A client is an interface with comment directives. `avero generate` writes the
implementation.

## Steps

1. Write the interface in `internal/clients/<service>/<service>.go`.
2. Write the directive above the interface:

   ```go
   //avero:client base="https://api.example.com" auth="bearer" timeout="5s"
   type Payments interface {
       //avero:GET /charges/{id}
       GetCharge(ctx context.Context, id string) (Charge, error)
   }
   ```

3. Give each method a directive with the HTTP method and the path. A path
   parameter in braces binds to the method parameter of the same name.
4. Run `avero generate`.
5. Write the test against `httptest`. Pass `client.WithBaseURL(srv.URL)` to the
   generated constructor.
6. Supply the credential in `main.go` with `client.WithToken`.

## The command that proves the work

```
go test ./internal/clients/... -race
```
