// Package payments holds the client of the payment service. The interface and
// its directives are the declaration. `avero generate` writes the
// implementation beside this file. See S13.
package payments

import "context"

//go:generate go run github.com/alternayte/avero/internal/cmd/averogen .

// Charge is one payment.
type Charge struct {
	// ID identifies the charge.
	ID string `json:"id"`
	// Amount is the value in the smallest unit of the currency.
	Amount int64 `json:"amount"`
	// Currency is the three letter code.
	Currency string `json:"currency"`
}

// NewCharge is the body of a create call.
type NewCharge struct {
	// Amount is the value in the smallest unit of the currency.
	Amount int64 `json:"amount"`
	// Currency is the three letter code.
	Currency string `json:"currency"`
}

// Payments reads and writes the charges of the payment service.
//
// The runtime carries the timeout, the retry with backoff, the circuit breaker
// and one OTel span for each call. Pass client.WithToken in main.go to supply
// the credential, so no secret reaches the source.
//
//avero:client base="https://api.payments.example.com" auth="bearer" timeout="5s" attempts="3"
type Payments interface {
	//avero:GET /charges/{id}
	GetCharge(ctx context.Context, id string) (Charge, error)

	//avero:POST /charges
	CreateCharge(ctx context.Context, body NewCharge) (*Charge, error)
}
