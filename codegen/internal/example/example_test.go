package example_test

import (
	"encoding/json"
	"go/parser"
	"go/token"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/alternayte/avero/codegen/internal/example"
	"github.com/alternayte/avero/router"
)

// The generated code is committed, so these tests compile and run it. They
// cover the binding precedence and every rule that the SDD names.

const validUUID = "0f8fad5b-d9cb-469f-a165-70867728950e"

// handler builds a router that binds CreateInput and echoes it.
func handler(t *testing.T) http.Handler {
	t.Helper()
	m := &example.Module{}
	r := router.New()
	r.Post("/boards/{board_id}/cards", router.In(m.Create))
	r.Get("/cards", router.In(m.List))
	h, err := r.Handler()
	if err != nil {
		t.Fatalf("Handler returned an error: %v", err)
	}
	return h
}

// cards returns the URL of the create route. Every request carries page,
// because Page holds a min rule and a page number starts at one.
func cards(extra string) string {
	u := "/boards/" + validUUID + "/cards?page=1"
	if extra != "" {
		u += "&" + extra
	}
	return u
}

// post performs one request and returns the recorder.
func post(t *testing.T, h http.Handler, target, contentType, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, target, strings.NewReader(body))
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

// valid returns a form body that passes every rule.
func valid() string {
	return "title=a good title&email=person@example.com&role=member"
}

// decode reads the echoed input.
func decode(t *testing.T, rec *httptest.ResponseRecorder) example.CreateInput {
	t.Helper()
	var in example.CreateInput
	if err := json.Unmarshal(rec.Body.Bytes(), &in); err != nil {
		t.Fatalf("the body does not parse: %v\n%s", err, rec.Body.String())
	}
	return in
}

// errorsOf reads the field errors of a 422.
func errorsOf(t *testing.T, rec *httptest.ResponseRecorder) map[string]string {
	t.Helper()
	var body struct {
		Errors map[string]string `json:"errors"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("the body does not parse: %v\n%s", err, rec.Body.String())
	}
	return body.Errors
}

func TestTheGeneratedFileDoesNotImportReflect(t *testing.T) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "zz_generated.go", nil, parser.ImportsOnly)
	if err != nil {
		t.Fatalf("the generated file does not parse: %v", err)
	}
	for _, imp := range file.Imports {
		if imp.Path.Value == `"reflect"` {
			t.Fatal("the generated file imports reflect")
		}
	}
}

func TestBindingReadsThePathTheFormAndTheQuery(t *testing.T) {
	h := handler(t)
	rec := post(t, h, cards("size=9&ratio=1.5&timeout=15s&tags=a&tags=b"),
		"application/x-www-form-urlencoded", valid())
	if rec.Code != 201 {
		t.Fatalf("gave %d %s", rec.Code, rec.Body.String())
	}
	in := decode(t, rec)
	if in.BoardID != validUUID {
		t.Fatalf("BoardID is %q", in.BoardID)
	}
	if in.Title != "a good title" {
		t.Fatalf("Title is %q", in.Title)
	}
	if in.Page != 1 || in.Size != 9 || in.Ratio != 1.5 {
		t.Fatalf("the numbers are %d %d %v", in.Page, in.Size, in.Ratio)
	}
	if in.Timeout != 15*time.Second {
		t.Fatalf("Timeout is %v", in.Timeout)
	}
	if len(in.Tags) != 2 || in.Tags[0] != "a" || in.Tags[1] != "b" {
		t.Fatalf("Tags is %v", in.Tags)
	}
}

func TestBindingReadsTheJSONBody(t *testing.T) {
	h := handler(t)
	body := `{"title":"from the body","email":"person@example.com","role":"admin","notify":true,"tags":["x"]}`
	rec := post(t, h, cards(""), "application/json", body)
	if rec.Code != 201 {
		t.Fatalf("gave %d %s", rec.Code, rec.Body.String())
	}
	in := decode(t, rec)
	if in.Title != "from the body" || !in.Notify || len(in.Tags) != 1 {
		t.Fatalf("the input is %+v", in)
	}
}

func TestThePathWinsOverEveryOtherSource(t *testing.T) {
	// board_id is a path wildcard. A form field of the same name must not
	// replace it.
	h := handler(t)
	rec := post(t, h, cards(""),
		"application/x-www-form-urlencoded", valid()+"&board_id=not-the-path")
	in := decode(t, rec)
	if in.BoardID != validUUID {
		t.Fatalf("BoardID is %q, want the path value", in.BoardID)
	}
}

func TestTheQueryWinsOverTheForm(t *testing.T) {
	h := handler(t)
	rec := post(t, h, cards("notify=true"),
		"application/x-www-form-urlencoded", valid()+"&notify=false")
	if in := decode(t, rec); !in.Notify {
		t.Fatal("the form replaced the query")
	}
}

func TestTheFormWinsOverTheBody(t *testing.T) {
	// A request that carries both is unusual, but the order must hold.
	h := handler(t)
	req := httptest.NewRequest(http.MethodPost, cards(""),
		strings.NewReader(valid()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if in := decode(t, rec); in.Title != "a good title" {
		t.Fatalf("Title is %q", in.Title)
	}
}

func TestASkippedFieldNeverBinds(t *testing.T) {
	h := handler(t)
	rec := post(t, h, cards("computed=leak"),
		"application/x-www-form-urlencoded", valid()+"&computed=leak")
	if in := decode(t, rec); in.Computed != "" {
		t.Fatalf("Computed is %q, want the empty string", in.Computed)
	}
}

func TestAMalformedNumberReturns400(t *testing.T) {
	h := handler(t)
	rec := post(t, h, "/boards/"+validUUID+"/cards?page=many",
		"application/x-www-form-urlencoded", valid())
	if rec.Code != 400 {
		t.Fatalf("gave %d, want 400", rec.Code)
	}
}

// The rules that the SDD names. Each one has a test.

func TestRuleRequired(t *testing.T) {
	h := handler(t)
	rec := post(t, h, cards(""),
		"application/x-www-form-urlencoded", "email=person@example.com&role=member")
	if rec.Code != 422 {
		t.Fatalf("gave %d, want 422", rec.Code)
	}
	if got := errorsOf(t, rec)["Title"]; got != "is required" {
		t.Fatalf("Title reads %q", got)
	}
}

func TestRuleMinOnAString(t *testing.T) {
	h := handler(t)
	rec := post(t, h, cards(""),
		"application/x-www-form-urlencoded", "title=ab&email=person@example.com&role=member")
	if got := errorsOf(t, rec)["Title"]; !strings.Contains(got, "at least 3") {
		t.Fatalf("Title reads %q", got)
	}
}

func TestRuleMaxOnAString(t *testing.T) {
	h := handler(t)
	long := strings.Repeat("x", 51)
	rec := post(t, h, cards(""),
		"application/x-www-form-urlencoded", "title="+long+"&email=person@example.com&role=member")
	if got := errorsOf(t, rec)["Title"]; !strings.Contains(got, "at most 50") {
		t.Fatalf("Title reads %q", got)
	}
}

func TestRuleMinAndMaxOnANumber(t *testing.T) {
	h := handler(t)
	rec := post(t, h, "/boards/"+validUUID+"/cards?page=0",
		"application/x-www-form-urlencoded", valid())
	if got := errorsOf(t, rec)["Page"]; !strings.Contains(got, "1 or more") {
		t.Fatalf("Page reads %q", got)
	}
	rec = post(t, h, "/boards/"+validUUID+"/cards?page=101",
		"application/x-www-form-urlencoded", valid())
	if got := errorsOf(t, rec)["Page"]; !strings.Contains(got, "100 or less") {
		t.Fatalf("Page reads %q", got)
	}
}

func TestRuleEmail(t *testing.T) {
	h := handler(t)
	rec := post(t, h, cards(""),
		"application/x-www-form-urlencoded", "title=a good title&email=not-an-address&role=member")
	if got := errorsOf(t, rec)["Email"]; got != "must be an email address" {
		t.Fatalf("Email reads %q", got)
	}
}

func TestRuleUUID(t *testing.T) {
	h := handler(t)
	rec := post(t, h, "/boards/not-a-uuid/cards?page=1",
		"application/x-www-form-urlencoded", valid())
	if got := errorsOf(t, rec)["BoardID"]; got != "must be a UUID" {
		t.Fatalf("BoardID reads %q", got)
	}
}

func TestRuleOneOf(t *testing.T) {
	h := handler(t)
	rec := post(t, h, cards(""),
		"application/x-www-form-urlencoded", "title=a good title&email=person@example.com&role=owner")
	if got := errorsOf(t, rec)["Role"]; !strings.Contains(got, "admin, member, viewer") {
		t.Fatalf("Role reads %q", got)
	}
}

func TestTheCustomRule(t *testing.T) {
	h := handler(t)
	rec := post(t, h, cards(""),
		"application/x-www-form-urlencoded", "title=admin&email=person@example.com&role=member")
	if got := errorsOf(t, rec)["Title"]; got != "must not be a reserved word" {
		t.Fatalf("Title reads %q", got)
	}
}

func TestTheCustomRuleRunsAfterTheTagRules(t *testing.T) {
	// The tag rule wins, because the first message of a field stays.
	h := handler(t)
	rec := post(t, h, cards(""),
		"application/x-www-form-urlencoded", "title=&email=person@example.com&role=member")
	if got := errorsOf(t, rec)["Title"]; got != "is required" {
		t.Fatalf("Title reads %q", got)
	}
}

func TestEveryFailedFieldIsReported(t *testing.T) {
	h := handler(t)
	rec := post(t, h, "/boards/not-a-uuid/cards?page=1",
		"application/x-www-form-urlencoded", "title=ab&email=nope&role=owner")
	got := errorsOf(t, rec)
	for _, field := range []string{"BoardID", "Title", "Email", "Role"} {
		if got[field] == "" {
			t.Fatalf("the report drops %s: %v", field, got)
		}
	}
}

// BindAllocationBound is the number of allocations that one call of the
// generated Bind and Validate may cost.
//
// The measurement covers the generated code alone. The request is built and
// parsed one time outside the measurement, because building a request is the
// cost of net/http and not the cost of the generator.
//
// CreateInput reads four sources, converts eight types and applies nine rules.
// The measured cost is 13, and most of it is the one parse of the query
// string. The bound holds a small margin for a change in the standard library.
// Raise this number only with a reason in the commit message.
const BindAllocationBound = 15

// preparedRequest returns a request whose form is already parsed, so that the
// measurement covers the generated code and not net/http.
func preparedRequest(t testing.TB) *router.Ctx {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, cards("size=9&timeout=15s&tags=a&tags=b"),
		strings.NewReader(valid()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetPathValue("board_id", validUUID)
	if err := req.ParseForm(); err != nil {
		t.Fatalf("ParseForm returned an error: %v", err)
	}
	return router.NewCtx(httptest.NewRecorder(), req)
}

func TestBindingStaysInsideItsAllocationBound(t *testing.T) {
	c := preparedRequest(t)
	got := testing.AllocsPerRun(500, func() {
		var in example.CreateInput
		if err := in.Bind(c); err != nil {
			t.Fatalf("Bind returned an error: %v", err)
		}
		var f router.Fields
		in.Validate(c, &f)
	})
	if got > BindAllocationBound {
		t.Fatalf("one binding cost %v allocations, and the bound is %d", got, BindAllocationBound)
	}
	t.Logf("one binding costs %v allocations, and the bound is %d", got, BindAllocationBound)
}

// BenchmarkBind measures the generated code alone.
func BenchmarkBind(b *testing.B) {
	c := preparedRequest(b)
	b.ReportAllocs()
	for b.Loop() {
		var in example.CreateInput
		if err := in.Bind(c); err != nil {
			b.Fatalf("Bind returned an error: %v", err)
		}
		var f router.Fields
		in.Validate(c, &f)
	}
}

// BenchmarkRequest measures a whole request, so that a person compares the
// cost of the generator with the cost of net/http.
func BenchmarkRequest(b *testing.B) {
	m := &example.Module{}
	r := router.New()
	r.Post("/boards/{board_id}/cards", router.In(m.Create))
	h, err := r.Handler()
	if err != nil {
		b.Fatalf("Handler returned an error: %v", err)
	}
	body := valid()
	b.ReportAllocs()
	for b.Loop() {
		req := httptest.NewRequest(http.MethodPost, cards(""), strings.NewReader(body))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		h.ServeHTTP(httptest.NewRecorder(), req)
	}
}
