// Package example holds the client that the tests of the generator drive. The
// generated file beside it is the output of `avero generate`. See the SDD,
// S13.
package example

//go:generate go run github.com/alternayte/avero/internal/cmd/averogen .

import "context"

// Thing is one row of the service.
type Thing struct {
	// ID identifies the thing.
	ID string `json:"id"`
	// Title is the name that a person reads.
	Title string `json:"title"`
}

// List is the answer of a list call.
type List struct {
	// Things holds the page.
	Things []Thing `json:"things"`
	// Total is the number of rows of the whole set.
	Total int `json:"total"`
}

// ListOptions holds the query of a list call and the headers of the request.
type ListOptions struct {
	// Page is the page number.
	Page int `query:"page"`
	// Search filters the set. An empty value sends no parameter.
	Search string `query:"q,omitempty"`
	// Full asks for the whole row.
	Full bool `query:"full"`
	// Limit bounds the page. A nil value sends no parameter.
	Limit *int `query:"limit"`
	// Key is the API key of the caller.
	Key string `header:"X-Api-Key"`
	// Trace names the call in the log of the service.
	Trace string `header:"X-Trace,omitempty"`
}

// NewThing is the body of a create call.
type NewThing struct {
	// Title is the name of the new thing.
	Title string `json:"title"`
}

// Example is the interface that the directive turns into a client. The
// generated implementation lives in the file beside this one.
//
//avero:client base="https://api.example.com" auth="bearer" timeout="5s" attempts="3" backoff="10ms"
type Example interface {
	//avero:GET /things/{id}
	GetThing(ctx context.Context, id string) (Thing, error)

	//avero:GET /things
	ListThings(ctx context.Context, opts ListOptions) (List, error)

	//avero:POST /things
	CreateThing(ctx context.Context, body NewThing) (*Thing, error)

	//avero:DELETE /things/{id}
	DeleteThing(ctx context.Context, id string) error

	//avero:GET /accounts/{account}/things/{id}
	GetAccountThing(ctx context.Context, account int, id string) (Thing, error)
}
