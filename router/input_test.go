package router_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/alternayte/avero/router"
)

// createInput stands in for a generated input. The generator writes Bind and
// Validate exactly like this, so the runtime contract is testable without it.
type createInput struct {
	Title string
	Page  int
}

func (in *createInput) Bind(c *router.Ctx) error {
	in.Title = c.Request().FormValue("title")
	if raw := c.Request().URL.Query().Get("page"); raw != "" {
		if raw == "many" {
			return errors.New("page is not a number")
		}
		in.Page = 2
	}
	return nil
}

func (in *createInput) Validate(_ *router.Ctx, f *router.Fields) {
	if in.Title == "" {
		f.Add("Title", "is required")
	}
}

func TestInBindsAndCallsTheHandler(t *testing.T) {
	var got createInput
	r := router.New()
	r.Post("/things", router.In(func(_ *router.Ctx, in createInput) (router.Response, error) {
		got = in
		return router.Text(201, "made"), nil
	}))

	h, err := r.Handler()
	if err != nil {
		t.Fatalf("Handler returned an error: %v", err)
	}
	req := newRequest(http.MethodPost, "/things", strings.NewReader("title=hello"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if rec := record(h, req); rec.Code != 201 {
		t.Fatalf("gave %d, want 201", rec.Code)
	}
	if got.Title != "hello" {
		t.Fatalf("the handler read %q", got.Title)
	}
}

func TestAValidationFaultReturns422WithAMapOfFieldToMessage(t *testing.T) {
	reached := false
	r := router.New()
	r.Post("/things", router.In(func(*router.Ctx, createInput) (router.Response, error) {
		reached = true
		return router.Text(201, "made"), nil
	}))

	rec := serve(t, r, http.MethodPost, "/things")
	if reached {
		t.Fatal("the handler ran although validation failed")
	}
	if rec.Code != 422 {
		t.Fatalf("gave %d, want 422", rec.Code)
	}
	var body struct {
		Errors map[string]string `json:"errors"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("the body does not parse: %v", err)
	}
	if body.Errors["Title"] != "is required" {
		t.Fatalf("the body carries %v", body.Errors)
	}
}

func TestABindFaultReturns400(t *testing.T) {
	r := router.New()
	r.Post("/things", router.In(func(*router.Ctx, createInput) (router.Response, error) {
		return router.NoContent(), nil
	}))
	h, err := r.Handler()
	if err != nil {
		t.Fatalf("Handler returned an error: %v", err)
	}
	if rec := record(h, newRequest(http.MethodPost, "/things?page=many", nil)); rec.Code != 400 {
		t.Fatalf("gave %d, want 400", rec.Code)
	}
}

func TestABindFaultDoesNotLeakItsMessage(t *testing.T) {
	r := router.New()
	r.Post("/things", router.In(func(*router.Ctx, createInput) (router.Response, error) {
		return router.NoContent(), nil
	}))
	h, _ := r.Handler()
	rec := record(h, newRequest(http.MethodPost, "/things?page=many", nil))
	if strings.Contains(rec.Body.String(), "page is not a number") {
		t.Fatalf("the response leaks the bind error: %q", rec.Body.String())
	}
}

func TestTheErrorsAndTheOldInputReachTheContext(t *testing.T) {
	// S10 renders the form again from these two values.
	var (
		fields *router.Fields
		old    *createInput
	)
	r := router.New()
	r.Use(router.Middleware{Name: "capture", Wrap: func(next router.Handler) router.Handler {
		return func(c *router.Ctx) (router.Response, error) {
			res, err := next(c)
			fields = router.FieldsFrom(c.Context())
			if v, ok := router.OldFrom(c.Context()).(*createInput); ok {
				old = v
			}
			return res, err
		}
	}})
	r.Post("/things", router.In(func(*router.Ctx, createInput) (router.Response, error) {
		return router.NoContent(), nil
	}))

	h, _ := r.Handler()
	req := newRequest(http.MethodPost, "/things", strings.NewReader("title="))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	record(h, req)

	if fields == nil || fields.Get("Title") != "is required" {
		t.Fatalf("the context carries no field error: %v", fields)
	}
	if old == nil {
		t.Fatal("the context carries no old input")
	}
}

func TestWithValidationResponseReplacesThe422(t *testing.T) {
	r := router.New(router.WithValidationResponse(
		func(_ *router.Ctx, f *router.Fields) router.Response {
			return router.Text(422, "the form has "+itoa(f.Len())+" faults")
		}))
	r.Post("/things", router.In(func(*router.Ctx, createInput) (router.Response, error) {
		return router.NoContent(), nil
	}))
	rec := serve(t, r, http.MethodPost, "/things")
	if rec.Body.String() != "the form has 1 faults" {
		t.Fatalf("the body is %q", rec.Body.String())
	}
}

func itoa(n int) string { return string(rune('0' + n)) }

func TestFieldsCollectsInOrderAndReportsMembership(t *testing.T) {
	var f router.Fields
	if f.Len() != 0 || f.Has("Title") {
		t.Fatal("an empty Fields is not empty")
	}
	f.Add("Title", "is required")
	f.Add("Email", "must be an email address")
	f.Add("Title", "the first message stays")

	if f.Len() != 2 {
		t.Fatalf("Len is %d, want 2", f.Len())
	}
	if !f.Has("Title") || f.Get("Title") != "is required" {
		t.Fatalf("Title reads %q", f.Get("Title"))
	}
	if f.Get("Absent") != "" {
		t.Fatalf("an absent field reads %q", f.Get("Absent"))
	}
	m := f.Map()
	if len(m) != 2 || m["Email"] != "must be an email address" {
		t.Fatalf("the map is %v", m)
	}
}

func TestFieldsMapIsACopy(t *testing.T) {
	var f router.Fields
	f.Add("Title", "is required")
	m := f.Map()
	m["Title"] = "changed"
	if f.Get("Title") != "is required" {
		t.Fatal("Map returned the inner map")
	}
}
