package avero

import (
	"strings"
	"testing"
)

// A wiring that states no router names the repair, so a person reads what to
// set. See DX-7.
func TestTheWiringStatesItsFaults(t *testing.T) {
	cases := []struct {
		name  string
		w     *Wiring
		holds string
	}{
		{"a nil wiring", nil, "*avero.Wiring"},
		{"no router", &Wiring{Modules: Modules()}, "Router"},
		{"no module set", &Wiring{Router: NewRouter()}, "Modules"},
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

// A whole wiring passes.
func TestTheWiringPasses(t *testing.T) {
	w := &Wiring{Router: NewRouter(), Modules: Modules()}
	if err := w.validate(); err != nil {
		t.Fatalf("validate failed: %v", err)
	}
}
