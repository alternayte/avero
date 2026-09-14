package cli

import (
	"bytes"
	"context"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/alternayte/avero/cli/pipeline"
	"github.com/alternayte/avero/codegen"
	"github.com/alternayte/avero/codegen/client"
	"github.com/alternayte/avero/internal/inspect"
	"github.com/alternayte/avero/openapi"
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
	project, err := LoadProject(dir)
	if err != nil {
		return fail(s, err)
	}
	ctx := context.Background()
	if check {
		for _, err := range []error{codegen.Check(dir), client.Check(dir), CheckDescription(ctx, dir, project)} {
			if err != nil {
				return fail(s, err)
			}
		}
		return 0
	}
	// drel owns the models and the migrations. templ owns the .templ files.
	// Both write Go files that the generators of Avero read.
	if err := Drel(ctx, dir); err != nil {
		return fail(s, err)
	}
	if err := Templ(ctx, dir); err != nil {
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
	// The description of the API and the client of the front end follow the
	// handlers of Go, so one command writes every generated file. `avero
	// generate --check` proves the same files.
	if err := Describe(ctx, dir, project, s); err != nil {
		return fail(s, err)
	}
	return 0
}

// DrelConfig is the file that states the models and the migrations of each
// feature slice.
const DrelConfig = "drel.yaml"

// Drel runs the drel generator when the application states its models.
//
// The application carries the generator as a tool of its go.mod, so `go tool
// drel` needs no second installation. drel writes the columns, the repository
// and the migration set of each model.
func Drel(ctx context.Context, dir string) error {
	body, err := os.ReadFile(filepath.Join(dir, DrelConfig))
	if err != nil {
		return nil
	}
	if !drelModules(string(body)) {
		// The file states no model package yet, and drel refuses an empty
		// modules block. An application that holds no slice therefore
		// generates its handlers and waits for `avero slice`.
		return nil
	}
	out, err := goCommand(ctx, dir, "tool", "drel", "generate")
	if err != nil {
		return fmt.Errorf("avero generate: drel failed\n%s%s", strings.TrimSpace(out),
			hint("Repair the model that the message names. Run the command again."))
	}
	return nil
}

// drelModules reports a drel.yaml that names one model package or more.
//
// drel answers "no packages specified" for an empty modules block, so Avero
// reads the block first and runs the generator only when a slice exists.
func drelModules(source string) bool {
	inModules := false
	for _, line := range strings.Split(source, "\n") {
		if strings.HasPrefix(line, "modules:") {
			inModules = true
			continue
		}
		if !inModules {
			continue
		}
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		if !strings.HasPrefix(line, " ") && !strings.HasPrefix(line, "\t") {
			// Another block of the file starts here.
			inModules = false
			continue
		}
		if strings.HasPrefix(trimmed, "- ") {
			return true
		}
	}
	return false
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
		return fmt.Errorf("avero generate: templ failed\n%s%s", strings.TrimSpace(out),
			hint("Repair the .templ file that the message names. Run the command again."))
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
	// generate writes the description of the API and the client of the front
	// end, so the build reads them.
	if code := runGenerate(ctx, s, nil); code != 0 {
		return code
	}
	if err := BuildAssets(ctx, dir, project, minify, s); err != nil {
		return fail(s, err)
	}
	out, goErr := goCommand(ctx, dir, "build", "-o", "bin/"+project.Name, ".")
	if goErr != nil {
		return failf(s, "avero build: the compiler failed\n%s\n  → Repair the fault that the compiler names. Run `avero build` again.", strings.TrimSpace(out))
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
	cfg := project.assetConfig(dir, minify)
	if project.Assets.Tier == "external" && len(cfg.Command) == 0 {
		// The application names no command, so the build runs the build
		// script of the package manager of the front end.
		tool, err := NodeToolOf(project, frontEnd(dir, project))
		if err != nil {
			return err
		}
		cfg.Command = tool.Script(ScriptBuild)
	}
	m, err := pipeline.Build(ctx, cfg)
	if err != nil {
		return err
	}
	if project.Assets.Tier == "external" {
		_, _ = fmt.Fprintf(s.Out, "assets: %s\n", strings.Join(cfg.Command, " "))
		return nil
	}
	_, _ = fmt.Fprintf(s.Out, "assets: %d files\n", m.Len())
	return nil
}

// Describe writes the description of the API and the client of the front end.
//
// The Go handlers state the routes, the input types and the answers. The
// description states the same, and the generator of the front end reads it, so
// one change of a handler reaches the types of the front end.
func Describe(ctx context.Context, dir string, project *Project, s Streams) error {
	doc, ok, err := description(ctx, dir, project)
	if err != nil || !ok {
		return err
	}
	name := filepath.Join(dir, OpenAPIFile)
	if err := os.WriteFile(name, doc, 0o644); err != nil {
		return fmt.Errorf("avero generate: %s does not write\n  → Give the process the right to write the application directory", OpenAPIFile)
	}
	_, _ = fmt.Fprintln(s.Out, OpenAPIFile)

	front := frontEnd(dir, project)
	tool, err := NodeToolOf(project, front)
	if err != nil {
		return err
	}
	if front == "" || !tool.HasScript(ScriptAPI) {
		// The front end states no generator of a client, so the description
		// is the whole answer.
		return nil
	}
	if err := Install(ctx, dir, project, s); err != nil {
		return err
	}
	command := tool.Script(ScriptAPI)
	cmd, err := tool.Command(ctx, command)
	if err != nil {
		return err
	}
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("avero generate: the client of the front end does not generate\n%s%s",
			strings.TrimSpace(string(out)),
			hint("Run `"+strings.Join(command, " ")+"` in "+project.Assets.Dir+" and repair the fault that it names."))
	}
	return nil
}

// CheckDescription reports a description of the API that is not current.
//
// The client of the front end follows openapi.json, so a description that
// drifts is the whole fault: the generated types of the front end state a
// route or a field that the service does not hold. The check writes nothing.
func CheckDescription(ctx context.Context, dir string, project *Project) error {
	doc, ok, err := description(ctx, dir, project)
	if err != nil || !ok {
		return err
	}
	name := filepath.Join(dir, OpenAPIFile)
	held, err := os.ReadFile(name)
	if err != nil {
		return fmt.Errorf("avero generate: %s is absent\n  → Run `avero generate`, then commit %s", OpenAPIFile, OpenAPIFile)
	}
	if !bytes.Equal(held, doc) {
		return fmt.Errorf("avero generate: %s is not current\n  → Run `avero generate`, then commit %s", OpenAPIFile, OpenAPIFile)
	}
	return nil
}

// description returns the bytes of the description of the API of the
// application. The second result reports an application that states one.
//
// An application states a description when openapi.json stands beside
// avero.json, or when its front end holds the configuration of a generator of
// a client. A service that states neither writes no file.
func description(ctx context.Context, dir string, project *Project) ([]byte, bool, error) {
	if !wantsDescription(dir, project) {
		return nil, false, nil
	}
	// The application states its own routes. See runOpenAPI.
	result, err := inspect.Run(ctx, dir, inspect.OpenAPI, false)
	if err != nil {
		return nil, false, err
	}
	if result.Code != 0 {
		return nil, false, fmt.Errorf("avero generate: the description of the API does not write\n%s%s",
			strings.TrimSpace(result.Stderr),
			hint("Repair the fault that the message names. Run the command again."))
	}
	doc, err := openapi.Parse([]byte(result.Stdout))
	if err != nil {
		return nil, false, err
	}
	return []byte(doc.String() + "\n"), true, nil
}

// wantsDescription reports an application that holds a description of its API.
func wantsDescription(dir string, project *Project) bool {
	if _, err := os.Stat(filepath.Join(dir, OpenAPIFile)); err == nil {
		return true
	}
	front := frontEnd(dir, project)
	if front == "" {
		return false
	}
	if _, err := os.Stat(filepath.Join(front, OpenAPIConfig)); err == nil {
		return true
	}
	tool, err := NodeToolOf(project, front)
	return err == nil && tool.HasScript(ScriptAPI)
}

// frontEnd returns the directory of the front end of an external bundler, or
// the empty string when the project holds none.
func frontEnd(dir string, project *Project) string {
	if project.Assets.Tier != "external" || project.Assets.Dir == "" {
		return ""
	}
	return filepath.Join(dir, filepath.FromSlash(project.Assets.Dir))
}

// The names that the description of the API carries.
const (
	// OpenAPIFile is the description that `avero build` writes.
	OpenAPIFile = "openapi.json"
	// OpenAPIConfig is the configuration of the generator of the front end.
	OpenAPIConfig = "openapi-ts.config.ts"
)

// Install reads the dependencies of an external bundler.
//
// It runs one time: a directory that already holds node_modules needs no
// second read. The package manager is the one that the application names, so
// an application of Bun never calls npm. See NodeToolOf.
func Install(ctx context.Context, dir string, project *Project, s Streams) error {
	front := frontEnd(dir, project)
	if front == "" {
		return nil
	}
	if _, err := os.Stat(filepath.Join(front, "package.json")); err != nil {
		return nil
	}
	if _, err := os.Stat(filepath.Join(front, "node_modules")); err == nil {
		return nil
	}
	tool, err := NodeToolOf(project, front)
	if err != nil {
		return err
	}
	command := tool.Install()
	_, _ = fmt.Fprintf(s.Out, "front end: %s\n", strings.Join(command, " "))
	cmd, err := tool.Command(ctx, command)
	if err != nil {
		return err
	}
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("avero build: %s failed in %s\n%s%s",
			strings.Join(command, " "), project.Assets.Dir, strings.TrimSpace(string(out)),
			hint("Repair the fault that the message names. Run the command again."))
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
	if err := pipeline.PinJS(ctx, pipeline.PinConfig{Dir: dirOf(s)}, name, url); err != nil {
		return fail(s, err)
	}
	lock, err := pipeline.LoadLock(dirOf(s))
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
	wrote, err := pipeline.Init(dir, project.Name)
	if err != nil {
		return fail(s, err)
	}
	if !wrote {
		_, _ = fmt.Fprintf(s.Out, "%s is already present\n", pipeline.PackageName)
		return 0
	}
	project.Assets.Tier = "node"
	if err := project.Save(dir); err != nil {
		return fail(s, err)
	}
	tool, err := NodeToolOf(project, dir)
	if err != nil {
		return fail(s, err)
	}
	_, _ = fmt.Fprintf(s.Out, "wrote %s. Run `%s add <package>` and then `avero build`\n",
		pipeline.PackageName, tool.Name)
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
