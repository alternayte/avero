package avero_test

import (
	"bytes"
	"strings"
	"testing"

	avero "github.com/alternayte/avero"
	"github.com/alternayte/avero/module"
	"github.com/alternayte/avero/router"
)

// invoices and orders both register GET /invoices.
type invoices struct{}

func (invoices) Name() string { return "invoices" }
func (invoices) Routes(r *router.Router) {
	r.Get("/invoices", func(*router.Ctx) (router.Response, error) { return router.NoContent(), nil })
}

type orders struct{}

func (orders) Name() string { return "orders" }
func (orders) Routes(r *router.Router) {
	r.Get("/invoices", func(*router.Ctx) (router.Response, error) { return router.NoContent(), nil })
}

// setType proves that the wrapper returns the alias of the module package.
var setType = func(s *module.Set) *avero.ModuleSet { return s }

func TestModulesAttachesThroughTheRootPackage(t *testing.T) {
	set := avero.Modules(invoices{})
	set = setType(set)
	r := avero.NewRouter()
	if err := set.Attach(r); err != nil {
		t.Fatalf("Attach returned %v, want nil", err)
	}
	rep, err := r.Report()
	if err != nil {
		t.Fatalf("Report returned %v, want nil", err)
	}
	if _, ok := rep.Route("GET", "/invoices"); !ok {
		t.Fatalf("the router holds no GET /invoices:\n%s", rep)
	}
}

func TestTwoModulesOnOnePatternExitOneAndNameBoth(t *testing.T) {
	err := avero.Modules(invoices{}, orders{}).Attach(avero.NewRouter())
	if err == nil {
		t.Fatal("Attach returned nil, want the fault")
	}
	var out bytes.Buffer
	if code := avero.Exit(&out, err); code != 1 {
		t.Fatalf("Exit returned %d, want 1", code)
	}
	for _, want := range []string{"invoices", "orders", "GET /invoices"} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("the output does not name %q:\n%s", want, out.String())
		}
	}
}
