package sse_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/alternayte/avero/internal/sse"
)

// plain is a writer that cannot flush.
type plain struct{ http.ResponseWriter }

func TestNewRefusesAWriterThatCannotFlush(t *testing.T) {
	_, err := sse.New(plain{httptest.NewRecorder()}, httptest.NewRequest(http.MethodGet, "/", nil))
	if err == nil {
		t.Fatal("New returned nil, want a fault")
	}
}

func TestSendWritesTheIdentifierAndTheRetry(t *testing.T) {
	rec := httptest.NewRecorder()
	s, err := sse.New(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if err != nil {
		t.Fatalf("New returned %v, want nil", err)
	}
	if err := s.Send(sse.Event{Name: "row", ID: "7", Retry: 2 * time.Second, Data: []string{"a"}}); err != nil {
		t.Fatalf("Send returned %v, want nil", err)
	}
	want := "event: row\nid: 7\nretry: 2000\ndata: a\n\n"
	if rec.Body.String() != want {
		t.Fatalf("the stream wrote %q, want %q", rec.Body.String(), want)
	}
}

func TestSendSplitsALineThatHoldsANewLine(t *testing.T) {
	rec := httptest.NewRecorder()
	s, _ := sse.New(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if err := s.Send(sse.Event{Data: []string{"one\r\ntwo"}}); err != nil {
		t.Fatalf("Send returned %v, want nil", err)
	}
	if !strings.Contains(rec.Body.String(), "data: one\ndata: two\n") {
		t.Fatalf("the stream wrote %q", rec.Body.String())
	}
}
