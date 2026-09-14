package avero

import (
	"net/http"
	"strings"
	"testing"
)

// A wiring that states no routes names the repair, so a person reads what to
// set. See DX-7.
func TestTheWiringStatesItsFaults(t *testing.T) {
	cases := []struct {
		name  string
		w     *Wiring
		holds string
	}{
		{"a nil wiring", nil, "*avero.Wiring"},
		{"no routes", &Wiring{Modules: Modules()}, "Router"},
		{"a router and a handler", &Wiring{
			Router:  NewRouter(),
			Handler: http.NewServeMux(),
		}, "state one of the two"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := c.w.validate()
			if err == nil {
				t.Fatal("validate returned no error")
			}
			if !strings.Contains(err.Error(), c.holds) {
				t.Fatalf("the error does not name %q: %v", c.holds, err)
			}
		})
	}
}

// A wiring of the whole host passes, and so does a wiring that states a
// handler of another library and no module set.
func TestTheWiringPasses(t *testing.T) {
	for _, c := range []struct {
		name string
		w    *Wiring
	}{
		{"the whole host", &Wiring{Router: NewRouter(), Modules: Modules()}},
		{"a router alone", &Wiring{Router: NewRouter()}},
		{"a handler of another library", &Wiring{Handler: http.NewServeMux()}},
	} {
		t.Run(c.name, func(t *testing.T) {
			if err := c.w.validate(); err != nil {
				t.Fatalf("validate failed: %v", err)
			}
			if _, err := c.w.handler(); err != nil {
				t.Fatalf("handler failed: %v", err)
			}
		})
	}
}

// An application that states no module set holds no migration of a module and
// no contribution, and it reports neither.
func TestAWiringWithNoModuleSetReports(t *testing.T) {
	w := &Wiring{Handler: http.NewServeMux()}
	if sets := w.Modules.Migrations(); len(sets) != 0 {
		t.Fatalf("the wiring holds %d migration sets", len(sets))
	}
	rep, err := w.Modules.Report()
	if err != nil {
		t.Fatalf("Report returned %v", err)
	}
	if len(rep.Modules) != 0 {
		t.Fatalf("the report holds %d modules", len(rep.Modules))
	}
	if models := Schema(w.Modules); len(models.Models) != 0 {
		t.Fatalf("the schema holds %d models", len(models.Models))
	}
}
