# HTTP clients

A client is an interface with comment directives. `avero generate` writes the
implementation beside it.

## The declaration

```go
//avero:client base="https://api.payments.example.com" auth="bearer" timeout="5s" attempts="3"
type Payments interface {
	//avero:GET /charges/{id}
	GetCharge(ctx context.Context, id string) (Charge, error)

	//avero:POST /charges
	CreateCharge(ctx context.Context, body NewCharge) (*Charge, error)
}
```

| Option | Meaning |
|---|---|
| `base` | the address of the service. It is required. |
| `auth` | `bearer`, `basic` or `none` |
| `timeout` | the limit of one attempt, such as `5s` |
| `attempts` | the number of attempts, the first one included |
| `backoff` | the delay before the second attempt |

A method directive names the HTTP method and the path. Write
`idempotent="true"` to allow a retry of a method that carries a body.

## The parameters

- A placeholder of the path binds to the parameter of the same name.
- A struct with `query` tags or `header` tags becomes the query and the
  headers. A field with `omitempty`, or a pointer field, is sent only when it
  holds a value.
- Any other struct becomes the JSON body.
- The results are `(T, error)` or `error`.

## The construction

```go
payments, err := payments.NewPayments(
	client.WithToken(cfg.PaymentsToken.Reveal()),
)
```

The directive states the address, and an option after it replaces a default, so
a test points the client at `httptest`.

## The behaviour

The runtime carries the timeout of each attempt, the retry with doubling
backoff, the circuit breaker and one OTel client span for each call. A retry
runs for an idempotent method only, and `Retry-After` wins over the backoff.

The circuit breaker holds three states. Closed counts the failures in a row.
The failure that reaches the threshold opens the circuit. Open fails at once
until the cooldown ends, and then one probe decides.

## The errors

```go
var status *client.StatusError
if errors.As(err, &status) && status.Status == http.StatusNotFound {
	// status.Body and status.Response hold the answer.
}
```

A status outside the two hundreds becomes a `StatusError`, which keeps the body
and the response. A transport fault becomes a `TransportError`, which unwraps,
so `errors.Is(err, context.DeadlineExceeded)` holds after a timeout.

The generated file imports no reflect.
