package avero_test

import (
	"os/exec"
	"strings"
	"testing"
)

// The hypermedia adapters are optional imports. The root package must not pull
// them in, so an application that renders plain HTML carries neither library
// and neither shape. See the SDD, S12.
func TestTheRootPackageDependsOnNeitherAdapter(t *testing.T) {
	out, err := exec.Command("go", "list", "-deps", ".").CombinedOutput()
	if err != nil {
		t.Fatalf("go list failed: %v\n%s", err, out)
	}
	for _, name := range []string{
		"github.com/alternayte/avero/ds",
		"github.com/alternayte/avero/htmx",
	} {
		for _, line := range strings.Split(string(out), "\n") {
			if strings.TrimSpace(line) == name {
				t.Fatalf("the root package depends on %s, which must stay an optional import", name)
			}
		}
	}
}

// The view package must not depend on an adapter either, so the scaffolded ui
// package can import view and stay free of both libraries.
func TestTheViewPackageDependsOnNeitherAdapter(t *testing.T) {
	out, err := exec.Command("go", "list", "-deps", "./view").CombinedOutput()
	if err != nil {
		t.Fatalf("go list failed: %v\n%s", err, out)
	}
	for _, name := range []string{"/ds", "/htmx"} {
		for _, line := range strings.Split(string(out), "\n") {
			if strings.HasSuffix(strings.TrimSpace(line), "avero"+name) {
				t.Fatalf("the view package depends on %s", line)
			}
		}
	}
}
