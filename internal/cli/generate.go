package cli

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/alternayte/avero/assets"
	"github.com/alternayte/avero/codegen"
	"github.com/alternayte/avero/codegen/client"
)

// runGenerate writes the generated files of the application. The check flag
// reports a file that is not current and writes nothing.
func runGenerate(_ context.Context, s Streams, args []string) int {
	check := false
	for _, a := range args {
		switch a {
		case "--check", "-check":
			check = true
		default:
			return failf(s, "avero generate: the flag %q is not known\n  → Run `avero generate` or `avero generate --check`", a)
		}
	}
	dir := dirOf(s)
	if check {
		for _, err := range []error{codegen.Check(dir), client.Check(dir)} {
			if err != nil {
				return fail(s, err)
			}
		}
		return 0
	}
	// templ owns the .templ files. It writes the Go file of each one, and the
	// generators of Avero read the result.
	if err := Templ(context.Background(), dir); err != nil {
		return fail(s, err)
	}
	handlers, err := codegen.Generate(dir)
	if err != nil {
		return fail(s, err)
	}
	clients, err := client.Generate(dir)
	if err != nil {
		return fail(s, err)
	}
	for _, path := range append(handlers, clients...) {
		_, _ = fmt.Fprintln(s.Out, path)
	}
	return 0
}

// Templ runs the templ generator when the application holds a .templ file.
//
// The application carries the generator as a tool of its go.mod, so
// `go tool templ` needs no second installation. See the SDD, S15.
func Templ(ctx context.Context, dir string) error {
	if !templFiles(dir) {
		return nil
	}
	out, err := goCommand(ctx, dir, "tool", "templ", "generate")
	if err != nil {
		return fmt.Errorf("avero generate: templ failed\n%s\n  → Repair the .templ file that the message names, then run the command again",
			strings.TrimSpace(out))
	}
	return nil
}

// templFiles reports a tree that holds a .templ file.
func templFiles(dir string) bool {
	found := false
	_ = filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			name := d.Name()
			if path != dir && (name == "node_modules" || name == "vendor" || strings.HasPrefix(name, ".")) {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.EqualFold(filepath.Ext(path), ".templ") {
			found = true
			return filepath.SkipAll
		}
		return nil
	})
	return found
}

// runBuild generates, builds the assets and compiles the binary.
func runBuild(ctx context.Context, s Streams, args []string) int {
	minify := false
	for _, a := range args {
		switch a {
		case "--minify", "-minify":
			minify = true
		default:
			return failf(s, "avero build: the flag %q is not known\n  → Run `avero build` or `avero build --minify`", a)
		}
	}
	dir := dirOf(s)
	project, err := LoadProject(dir)
	if err != nil {
		return fail(s, err)
	}
	if code := runGenerate(ctx, s, nil); code != 0 {
		return code
	}
	if err := BuildAssets(ctx, dir, project, minify, s); err != nil {
		return fail(s, err)
	}
	out, goErr := goCommand(ctx, dir, "build", "-o", "bin/"+project.Name, ".")
	if goErr != nil {
		return failf(s, "avero build: the compiler failed\n%s\n  → Repair the fault that the compiler names, then run `avero build` again", strings.TrimSpace(out))
	}
	_, _ = fmt.Fprintf(s.Out, "binary: bin/%s\n", project.Name)
	return 0
}

// BuildAssets builds the front end of the application.
//
// A project with an external bundler reads its dependencies first, so one
// command builds an application that a person just wrote.
func BuildAssets(ctx context.Context, dir string, project *Project, minify bool, s Streams) error {
	if project.Assets.Tier != "external" &&
		len(project.Assets.Entries) == 0 && project.Assets.Tailwind.Input == "" {
		return nil
	}
	if err := Install(ctx, dir, project, s); err != nil {
		return err
	}
	m, err := assets.Build(ctx, project.assetConfig(dir, minify))
	if err != nil {
		return err
	}
	if project.Assets.Tier == "external" {
		_, _ = fmt.Fprintf(s.Out, "assets: %s\n", strings.Join(project.Assets.Command, " "))
		return nil
	}
	_, _ = fmt.Fprintf(s.Out, "assets: %d files\n", m.Len())
	return nil
}

// Install reads the dependencies of an external bundler.
//
// It runs one time: a directory that already holds node_modules needs no
// second read.
func Install(ctx context.Context, dir string, project *Project, s Streams) error {
	if project.Assets.Tier != "external" {
		return nil
	}
	front := filepath.Join(dir, filepath.FromSlash(project.Assets.Dir))
	if _, err := os.Stat(filepath.Join(front, "package.json")); err != nil {
		return nil
	}
	if _, err := os.Stat(filepath.Join(front, "node_modules")); err == nil {
		return nil
	}
	_, _ = fmt.Fprintln(s.Out, "front end: npm install")
	cmd := exec.CommandContext(ctx, "npm", "install", "--no-audit", "--no-fund")
	cmd.Dir = front
	cmd.Env = os.Environ()
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("avero build: npm install failed in %s\n%s\n  → Install Node.js, which the spa shape needs, then run the command again",
			project.Assets.Dir, strings.TrimSpace(string(out)))
	}
	return nil
}

// runJS fetches a bundled module into the vendor directory.
func runJS(ctx context.Context, s Streams, args []string) int {
	if len(args) < 2 || args[0] != "pin" {
		return failf(s, "avero js: the form is `avero js pin <package> [url]`\n  → Name the package, such as `avero js pin zustand`")
	}
	name := args[1]
	url := ""
	if len(args) > 2 {
		url = args[2]
	}
	if err := assets.PinJS(ctx, assets.PinConfig{Dir: dirOf(s)}, name, url); err != nil {
		return fail(s, err)
	}
	lock, err := assets.LoadLock(dirOf(s))
	if err != nil {
		return fail(s, err)
	}
	pin, _ := lock.JS(name)
	_, _ = fmt.Fprintf(s.Out, "pinned %s from %s\n\nImport it by its name:\n\n\timport ... from %q\n",
		name, pin.URL, name)
	return 0
}

// runAssets writes the package.json of tier 1.
func runAssets(_ context.Context, s Streams, args []string) int {
	if len(args) == 0 || args[0] != "init" {
		return failf(s, "avero assets: the form is `avero assets init`\n  → Run `avero assets init` to write the package.json of tier 1")
	}
	dir := dirOf(s)
	project, err := LoadProject(dir)
	if err != nil {
		return fail(s, err)
	}
	wrote, err := assets.Init(dir, project.Name)
	if err != nil {
		return fail(s, err)
	}
	if !wrote {
		_, _ = fmt.Fprintf(s.Out, "%s is already present\n", assets.PackageName)
		return 0
	}
	project.Assets.Tier = "node"
	if err := project.Save(dir); err != nil {
		return fail(s, err)
	}
	_, _ = fmt.Fprintf(s.Out, "wrote %s. Run `npm install <package>` and then `avero build`\n", assets.PackageName)
	return 0
}

// goCommand runs the Go tool in the directory of the application.
func goCommand(ctx context.Context, dir string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "go", args...)
	cmd.Dir = dir
	cmd.Env = os.Environ()
	out, err := cmd.CombinedOutput()
	return string(out), err
}
