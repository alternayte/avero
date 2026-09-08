package module_test

import (
	"context"
	"io/fs"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/alternayte/avero/module"
	"github.com/alternayte/avero/router"
)

// bare implements Module and no optional interface.
type bare struct{}

func (bare) Name() string { return "bare" }

// full implements every optional interface.
type full struct{}

func (full) Name() string { return "billing" }

func (full) Routes(r *router.Router) {
	r.Get("/invoices", func(*router.Ctx) (router.Response, error) { return router.NoContent(), nil })
	r.Post("/invoices", func(*router.Ctx) (router.Response, error) { return router.NoContent(), nil })
}

func (full) InboxHandlers() []module.InboxHandler {
	return []module.InboxHandler{handler{name: "OnCaptured", message: "payments.Captured"}}
}

func (full) Projections() []module.Projection {
	return []module.Projection{projection{name: "invoice_list"}}
}

func (full) Schedule(s *module.Scheduler) {
	s.Add(module.Job{Name: "sweep", Every: "1m", Run: func(context.Context) error { return nil }})
}

func (full) Migrations() fs.FS {
	return fstest.MapFS{"0001_invoices.sql": &fstest.MapFile{Data: []byte("select 1")}}
}

func (full) Describe() module.Description {
	return module.Description{
		Name:        "billing",
		Models:      []module.ModelDesc{{Name: "Invoice", Table: "invoices"}},
		Events:      []module.EventDesc{{Name: "InvoicePaid"}},
		Projections: []string{"invoice_list"},
		InboxTypes:  []string{"payments.Captured"},
	}
}

type handler struct{ name, message string }

func (h handler) HandlerName() string { return h.name }
func (h handler) MessageType() string { return h.message }

type projection struct{ name string }

func (p projection) ProjectionName() string { return p.name }

// clash registers the same pattern as full.
type clash struct{}

func (clash) Name() string { return "invoicing" }
func (clash) Routes(r *router.Router) {
	r.Get("/invoices", func(*router.Ctx) (router.Response, error) { return router.NoContent(), nil })
}

func TestAModuleWithNoOptionalInterfaceStartsWithoutAFault(t *testing.T) {
	set := module.Modules(bare{})
	if err := set.Err(); err != nil {
		t.Fatalf("Err returned %v, want nil", err)
	}
	r := router.New()
	if err := set.Attach(r); err != nil {
		t.Fatalf("Attach returned %v, want nil", err)
	}
	rep, err := set.Report()
	if err != nil {
		t.Fatalf("Report returned %v, want nil", err)
	}
	row, ok := rep.Module("bare")
	if !ok {
		t.Fatal("the report holds no row for bare")
	}
	if len(row.Interfaces) != 0 {
		t.Fatalf("Interfaces holds %v, want none", row.Interfaces)
	}
}

func TestTheTableListsEachModuleAndItsInterfaces(t *testing.T) {
	set := module.Modules(full{}, bare{})
	rep, err := set.Report()
	if err != nil {
		t.Fatalf("Report returned %v, want nil", err)
	}
	row, ok := rep.Module("billing")
	if !ok {
		t.Fatal("the report holds no row for billing")
	}
	want := []string{
		"DescribeModule", "HTTPModule", "InboxModule",
		"MigrationModule", "ProjectorModule", "ScheduleModule",
	}
	if strings.Join(row.Interfaces, ",") != strings.Join(want, ",") {
		t.Fatalf("Interfaces holds %v, want %v", row.Interfaces, want)
	}
	if len(row.Routes) != 2 {
		t.Fatalf("Routes holds %d rows, want 2", len(row.Routes))
	}
	if strings.Join(row.InboxHandlers, ",") != "OnCaptured" {
		t.Fatalf("InboxHandlers holds %v", row.InboxHandlers)
	}
	if strings.Join(row.Projections, ",") != "invoice_list" {
		t.Fatalf("Projections holds %v", row.Projections)
	}
	if strings.Join(row.Jobs, ",") != "sweep" {
		t.Fatalf("Jobs holds %v", row.Jobs)
	}
	if !row.Migrations {
		t.Fatal("Migrations reads false, want true")
	}
	table := rep.String()
	for _, name := range []string{"MODULE", "billing", "bare", "HTTPModule"} {
		if !strings.Contains(table, name) {
			t.Fatalf("the table does not hold %q:\n%s", name, table)
		}
	}
}

func TestAModuleInspectsOneTime(t *testing.T) {
	m := &counter{}
	set := module.Modules(m)
	if err := set.Attach(router.New()); err != nil {
		t.Fatalf("Attach returned %v, want nil", err)
	}
	if _, err := set.Report(); err != nil {
		t.Fatalf("Report returned %v, want nil", err)
	}
	if m.calls != 1 {
		t.Fatalf("Routes ran %d times, want 1", m.calls)
	}
}

type counter struct{ calls int }

func (c *counter) Name() string { return "counter" }
func (c *counter) Routes(r *router.Router) {
	c.calls++
	r.Get("/count", func(*router.Ctx) (router.Response, error) { return router.NoContent(), nil })
}

func TestTwoModulesOnOnePatternNameBothModules(t *testing.T) {
	set := module.Modules(full{}, clash{})
	err := set.Err()
	if err == nil {
		t.Fatal("Err returned nil, want a fault")
	}
	msg := err.Error()
	for _, want := range []string{"billing", "invoicing", "GET /invoices"} {
		if !strings.Contains(msg, want) {
			t.Fatalf("the fault does not name %q:\n%s", want, msg)
		}
	}
	if _, err := set.Report(); err == nil {
		t.Fatal("Report returned nil, want the fault")
	}
	if err := set.Attach(router.New()); err == nil {
		t.Fatal("Attach returned nil, want the fault")
	}
}

func TestAttachMountsTheRoutesOfEachModule(t *testing.T) {
	set := module.Modules(full{})
	r := router.New()
	r.Use(router.Middleware{Name: "requestid", Wrap: func(next router.Handler) router.Handler { return next }})
	if err := set.Attach(r); err != nil {
		t.Fatalf("Attach returned %v, want nil", err)
	}
	rep, err := r.Report()
	if err != nil {
		t.Fatalf("Report returned %v, want nil", err)
	}
	if len(rep.Routes) != 2 {
		t.Fatalf("the router holds %d routes, want 2", len(rep.Routes))
	}
	row, ok := rep.Route("GET", "/invoices")
	if !ok {
		t.Fatal("the router holds no GET /invoices")
	}
	if strings.Join(row.Middleware, ",") != "requestid" {
		t.Fatalf("Middleware holds %v, want requestid", row.Middleware)
	}
	if _, err := r.Handler(); err != nil {
		t.Fatalf("Handler returned %v, want nil", err)
	}
}

func TestAttachKeepsThePrefixOfAGroup(t *testing.T) {
	set := module.Modules(full{})
	r := router.New()
	var err error
	r.Group("/api", func(g *router.Router) { err = set.Attach(g) })
	if err != nil {
		t.Fatalf("Attach returned %v, want nil", err)
	}
	rep, repErr := r.Report()
	if repErr != nil {
		t.Fatalf("Report returned %v, want nil", repErr)
	}
	if _, ok := rep.Route("GET", "/api/invoices"); !ok {
		t.Fatalf("the router holds no GET /api/invoices:\n%s", rep)
	}
}

func TestTheReportHoldsTheDescription(t *testing.T) {
	rep, err := module.Modules(full{}, bare{}).Report()
	if err != nil {
		t.Fatalf("Report returned %v, want nil", err)
	}
	row, _ := rep.Module("billing")
	if row.Description == nil {
		t.Fatal("Description is nil, want the description of billing")
	}
	if len(row.Description.Routes) != 2 {
		t.Fatalf("the description holds %d routes, want 2", len(row.Description.Routes))
	}
	if row.Description.Models[0].Table != "invoices" {
		t.Fatalf("Models holds %v", row.Description.Models)
	}
	bareRow, _ := rep.Module("bare")
	if bareRow.Description == nil || bareRow.Description.Name != "bare" {
		t.Fatalf("the report holds no default description for bare: %v", bareRow.Description)
	}
}

func TestTheSchedulerAndTheMigrationsReachTheCaller(t *testing.T) {
	set := module.Modules(full{})
	if len(set.Jobs()) != 1 {
		t.Fatalf("Jobs holds %d rows, want 1", len(set.Jobs()))
	}
	if len(set.InboxHandlers()) != 1 {
		t.Fatalf("InboxHandlers holds %d rows, want 1", len(set.InboxHandlers()))
	}
	if len(set.Projections()) != 1 {
		t.Fatalf("Projections holds %d rows, want 1", len(set.Projections()))
	}
	if len(set.Migrations()) != 1 {
		t.Fatalf("Migrations holds %d file systems, want 1", len(set.Migrations()))
	}
}
