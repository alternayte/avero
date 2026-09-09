package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/alternayte/avero/codegen"
	"github.com/alternayte/avero/scaffold"
)

// runSlice writes a new feature slice.
func runSlice(ctx context.Context, s Streams, args []string) int {
	if len(args) == 0 {
		return failf(s, "avero slice: the feature has no name\n  → Run `avero slice <name>`, such as `avero slice comment`")
	}
	if len(args) > 1 || strings.HasPrefix(args[0], "-") {
		return failf(s, "avero slice: the command takes one name\n  → Run `avero slice <name>`, such as `avero slice comment`")
	}
	dir := dirOf(s)
	written, registered, err := Slice(ctx, dir, args[0])
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
func Slice(ctx context.Context, dir, name string) (written []string, registered bool, err error) {
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
	// drel reads the model package of the slice. It writes the columns and
	// the repository, then the first migration of the table.
	pkg := packageOf(name)
	if err := Drel(ctx, dir); err != nil {
		return nil, registered, err
	}
	if err := SliceMigration(ctx, dir, pkg); err != nil {
		return nil, registered, err
	}
	if _, err := codegen.Generate(dir); err != nil {
		return nil, registered, err
	}
	return written, registered, nil
}

// SliceMigration writes the first migration of one slice, then runs the drel
// generator again, so the binary carries the new migration set.
func SliceMigration(ctx context.Context, dir, pkg string) error {
	if _, err := os.Stat(filepath.Join(dir, DrelConfig)); err != nil {
		return nil
	}
	out, err := goCommand(ctx, dir, "tool", "drel", "migrate", "new", "--module", pkg, "create_"+pkg)
	if err != nil {
		return fmt.Errorf("avero slice: the first migration does not write\n%s\n  → Run `avero migrate new create_%s` and repair the fault that it names",
			strings.TrimSpace(out), pkg)
	}
	return Drel(ctx, dir)
}

// packageOf returns the package name of a feature name.
func packageOf(name string) string {
	name = strings.ToLower(strings.TrimSpace(name))
	return strings.TrimSuffix(name, "s") + "s"
}
