package payments_test

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/alternayte/avero/codegen/client"

	"orders/internal/clients/payments"
)

// serve builds the generated client against a test server.
func serve(t *testing.T, h http.Handler) payments.Payments {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	c, err := payments.NewPayments(client.WithBaseURL(srv.URL), client.WithToken("test"))
	if err != nil {
		t.Fatalf("NewPayments returned %v, want nil", err)
	}
	return c
}

func TestGetChargeReadsTheAnswer(t *testing.T) {
	c := serve(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/charges/7" {
			t.Errorf("the path is %q, want /charges/7", r.URL.Path)
		}
		_, _ = io.WriteString(w, `{"id":"7","amount":1200,"currency":"eur"}`)
	}))
	charge, err := c.GetCharge(context.Background(), "7")
	if err != nil {
		t.Fatalf("GetCharge returned %v, want nil", err)
	}
	if charge.Amount != 1200 {
		t.Fatalf("Amount = %d, want 1200", charge.Amount)
	}
}

func TestAnAbsentChargeMapsToATypedError(t *testing.T) {
	c := serve(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	_, err := c.GetCharge(context.Background(), "absent")
	var status *client.StatusError
	if !errors.As(err, &status) || status.Status != http.StatusNotFound {
		t.Fatalf("GetCharge returned %v, want a 404 status error", err)
	}
}
