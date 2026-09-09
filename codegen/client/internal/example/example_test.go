package example_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/alternayte/avero/codegen/client"
	"github.com/alternayte/avero/codegen/client/internal/example"
)

// serve builds the generated client against a test server.
func serve(t *testing.T, h http.Handler) example.Example {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	c, err := example.NewExample(client.WithBaseURL(srv.URL), client.WithToken("secret"))
	if err != nil {
		t.Fatalf("NewExample returned %v, want nil", err)
	}
	return c
}

func TestAPathParameterReachesTheServer(t *testing.T) {
	var path string
	c := serve(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.Path
		_, _ = io.WriteString(w, `{"id":"7","title":"a"}`)
	}))
	thing, err := c.GetThing(context.Background(), "7 8")
	if err != nil {
		t.Fatalf("GetThing returned %v, want nil", err)
	}
	if path != "/things/7 8" {
		t.Fatalf("the server read %q, want the escaped parameter", path)
	}
	if thing.Title != "a" {
		t.Fatalf("Title = %q", thing.Title)
	}
}

func TestTwoPathParametersReachTheServerInOrder(t *testing.T) {
	var path string
	c := serve(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.Path
		_, _ = io.WriteString(w, `{"id":"7"}`)
	}))
	if _, err := c.GetAccountThing(context.Background(), 42, "7"); err != nil {
		t.Fatalf("GetAccountThing returned %v, want nil", err)
	}
	if path != "/accounts/42/things/7" {
		t.Fatalf("the server read %q", path)
	}
}

func TestAQueryStructAndItsHeadersReachTheServer(t *testing.T) {
	var query, key, trace, auth string
	c := serve(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		query = r.URL.RawQuery
		key, trace = r.Header.Get("X-Api-Key"), r.Header.Get("X-Trace")
		auth = r.Header.Get("Authorization")
		_, _ = io.WriteString(w, `{"things":[{"id":"7"}],"total":1}`)
	}))
	limit := 20
	list, err := c.ListThings(context.Background(), example.ListOptions{
		Page: 2, Full: true, Limit: &limit, Key: "k",
	})
	if err != nil {
		t.Fatalf("ListThings returned %v, want nil", err)
	}
	if query != "full=true&limit=20&page=2" {
		t.Fatalf("the query is %q", query)
	}
	if key != "k" || trace != "" {
		t.Fatalf("the headers are %q %q, and an empty field sends no header", key, trace)
	}
	if auth != "Bearer secret" {
		t.Fatalf("Authorization = %q", auth)
	}
	if list.Total != 1 || len(list.Things) != 1 {
		t.Fatalf("the answer is %+v", list)
	}
}

func TestAJSONBodyReachesTheServer(t *testing.T) {
	var body, method, contentType string
	c := serve(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		body, method, contentType = string(raw), r.Method, r.Header.Get("Content-Type")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(example.Thing{ID: "7", Title: "a"})
	}))
	thing, err := c.CreateThing(context.Background(), example.NewThing{Title: "a"})
	if err != nil {
		t.Fatalf("CreateThing returned %v, want nil", err)
	}
	if method != http.MethodPost || contentType != "application/json" {
		t.Fatalf("the server read %q %q", method, contentType)
	}
	if body != `{"title":"a"}` {
		t.Fatalf("the body is %q", body)
	}
	if thing == nil || thing.ID != "7" {
		t.Fatalf("the answer is %+v", thing)
	}
}

func TestAMethodWithNoResultReadsNoBody(t *testing.T) {
	c := serve(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("the method is %s, want DELETE", r.Method)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	if err := c.DeleteThing(context.Background(), "7"); err != nil {
		t.Fatalf("DeleteThing returned %v, want nil", err)
	}
}

func TestAFourOhFourMapsToATypedErrorThatKeepsTheResponse(t *testing.T) {
	c := serve(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = io.WriteString(w, `{"error":"absent"}`)
	}))
	_, err := c.GetThing(context.Background(), "7")
	var status *client.StatusError
	if !errors.As(err, &status) {
		t.Fatalf("the error is %T, want *client.StatusError", err)
	}
	if status.Status != http.StatusNotFound {
		t.Fatalf("Status = %d, want 404", status.Status)
	}
	body, readErr := io.ReadAll(status.Response.Body)
	if readErr != nil || !strings.Contains(string(body), "absent") {
		t.Fatalf("the response body reads %q, %v", body, readErr)
	}
}

func TestTheGeneratedFileImportsNoReflect(t *testing.T) {
	body, err := os.ReadFile("zz_generated_client.go")
	if err != nil {
		t.Fatalf("ReadFile returned %v", err)
	}
	if strings.Contains(string(body), `"reflect"`) {
		t.Fatal("the generated file imports reflect")
	}
}
