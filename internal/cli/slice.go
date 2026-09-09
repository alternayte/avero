package cli

import (
	"context"
	"fmt"
	"strings"

	"github.com/alternayte/avero/codegen"
	"github.com/alternayte/avero/scaffold"
)

// runSlice writes a new feature slice.
func runSlice(_ context.Context, s Streams, args []string) int {
	if len(args) == 0 {
		return failf(s, "avero slice: the feature has no name\n  → Run `avero slice <name>`, such as `avero slice comment`")
	}
	if len(args) > 1 || strings.HasPrefix(args[0], "-") {
		return failf(s, "avero slice: the command takes one name\n  → Run `avero slice <name>`, such as `avero slice comment`")
	}
	dir := dirOf(s)
	written, registered, err := Slice(dir, args[0])
	if err != nil {
		return fail(s, err)
	}
	for _, path := range written {
		_, _ = fmt.Fprintln(s.Out, path)
	}
	if !registered {
		_, _ = fmt.Fprintf(s.Out, "\nRegister the module in wire.go:\n\n\tmodules := avero.Modules(%s.New(engine))\n", packageOf(args[0]))
	}
	_, _ = fmt.Fprintf(s.Out, "\nRun `avero migrate up`, then `avero verify`.\n")
	return 0
}

// Slice writes one feature slice, registers it and writes the generated files.
// The MCP tool scaffold_slice calls it, so an agent and a person write the same
// files. See the SDD, S16.
func Slice(dir, name string) (written []string, registered bool, err error) {
	written, err = scaffold.WriteSlice(scaffold.SliceOptions{Name: name, Dir: dir})
	if err != nil {
		return nil, false, err
	}
	module, err := scaffold.ModulePath(dir)
	if err != nil {
		return nil, false, err
	}
	registered, err = scaffold.RegisterSlice(dir, module, packageOf(name))
	if err != nil {
		return nil, false, err
	}
	if _, err := codegen.Generate(dir); err != nil {
		return nil, registered, err
	}
	return written, registered, nil
}

// packageOf returns the package name of a feature name.
func packageOf(name string) string {
	name = strings.ToLower(strings.TrimSpace(name))
	return strings.TrimSuffix(name, "s") + "s"
}
