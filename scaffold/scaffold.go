// Package scaffold writes a new Avero application, S14.
//
// One template set holds the files that every shape needs, and one set holds
// the files of each shape: ssr, spa and api. The scaffolder writes plain Go, so
// a person can read every file, change it and delete it. See design rule 4.
package scaffold

import (
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"text/template"
	"unicode"
)

//go:embed all:templates
var templates embed.FS

// The shapes that the scaffolder writes.
const (
	// ShapeSSR is a server rendered application.
	ShapeSSR = "ssr"
	// ShapeSPA is an application with an embedded single page front end.
	ShapeSPA = "spa"
	// ShapeAPI is a JSON service.
	ShapeAPI = "api"
)

// The hypermedia adapters that a shape can carry.
const (
	// Datastar is the default adapter.
	Datastar = "datastar"
	// Htmx is the other adapter.
	Htmx = "htmx"
)

// Options states one application.
type Options struct {
	// Name is the name of the application. It becomes the directory, the
	// module path and the binary.
	Name string
	// Dir is the directory that receives the files. An empty value is the
	// name inside the working directory.
	Dir string
	// Shape is ssr, spa or api.
	Shape string
	// Hypermedia is datastar or htmx. An api shape carries none.
	Hypermedia string
	// Module is the module path. An empty value is the name.
	Module string
	// AveroVersion is the version of the host that go.mod requires.
	AveroVersion string
	// TemplVersion is the version of the templ generator that an ssr
	// application requires.
	TemplVersion string
	// Replace points the module at a local copy of Avero. A test and the
	// gate of this repository set it. An empty value writes no replace
	// directive.
	Replace string
	// GoVersion is the version line of go.mod.
	GoVersion string
}

// data is the value that a template reads.
type data struct {
	Name       string
	Title      string
	Module     string
	Shape      string
	Hypermedia string
	Datastar   bool
	Htmx       bool
	SSR        bool
	SPA        bool
	API        bool
	Avero      string
	Templ      string
	Replace    string
	GoVersion  string
	Secret     string
}

// Write writes one application and returns the files that it wrote, in order.
//
// It refuses a directory that already holds files, so it never writes over the
// work of a person.
func Write(opts Options) ([]string, error) {
	if err := validate(&opts); err != nil {
		return nil, err
	}
	dir := opts.Dir
	if err := ensureEmpty(dir); err != nil {
		return nil, err
	}

	values := data{
		Name:       opts.Name,
		Title:      title(opts.Name),
		Module:     opts.Module,
		Shape:      opts.Shape,
		Hypermedia: opts.Hypermedia,
		Datastar:   opts.Hypermedia == Datastar,
		Htmx:       opts.Hypermedia == Htmx,
		SSR:        opts.Shape == ShapeSSR,
		SPA:        opts.Shape == ShapeSPA,
		API:        opts.Shape == ShapeAPI,
		Avero:      opts.AveroVersion,
		Templ:      opts.TemplVersion,
		Replace:    opts.Replace,
		GoVersion:  opts.GoVersion,
		Secret:     secret(),
	}

	// The agent files stand in their own set, because every shape carries
	// them and S16 states them. See the SDD, S16 part B.
	var written []string
	for _, set := range []string{"common", "agents", opts.Shape} {
		files, err := render(set, dir, values)
		if err != nil {
			return nil, err
		}
		written = append(written, files...)
	}
	if opts.Shape != ShapeAPI {
		files, err := writeStarterAssets(dir, opts.Shape)
		if err != nil {
			return nil, err
		}
		written = append(written, files...)
	}
	sort.Strings(written)
	return written, nil
}

// validate fills the defaults and refuses a wrong option.
func validate(opts *Options) error {
	if opts.Name == "" {
		return faultOf("the application has no name",
			"Run `avero new <name>`, such as `avero new blog`")
	}
	if !validName(opts.Name) {
		return faultOf(fmt.Sprintf("the name %q holds a character that a Go module path cannot carry", opts.Name),
			"Write a name of letters, digits, a dash and an underscore, such as `blog` or `order-service`")
	}
	switch opts.Shape {
	case "":
		opts.Shape = ShapeSSR
	case ShapeSSR, ShapeSPA, ShapeAPI:
	default:
		return faultOf(fmt.Sprintf("the shape %q is not known", opts.Shape),
			"Write --shape ssr, --shape spa or --shape api")
	}
	switch opts.Hypermedia {
	case "":
		opts.Hypermedia = Datastar
	case Datastar, Htmx:
	default:
		return faultOf(fmt.Sprintf("the hypermedia adapter %q is not known", opts.Hypermedia),
			"Write --hypermedia datastar or --hypermedia htmx")
	}
	if opts.Shape == ShapeAPI {
		// A JSON service renders no page, so it carries no adapter.
		opts.Hypermedia = ""
	}
	if opts.Module == "" {
		opts.Module = opts.Name
	}
	if opts.Dir == "" {
		opts.Dir = opts.Name
	}
	if opts.AveroVersion == "" {
		opts.AveroVersion = DefaultAveroVersion
	}
	if opts.TemplVersion == "" {
		opts.TemplVersion = DefaultTemplVersion
	}
	if opts.GoVersion == "" {
		opts.GoVersion = DefaultGoVersion
	}
	return nil
}

// The versions that go.mod carries when the caller names none.
const (
	// DefaultAveroVersion is the version of the host.
	DefaultAveroVersion = "v0.1.0"
	// DefaultGoVersion is the version line of go.mod.
	DefaultGoVersion = "1.26.2"
	// DefaultTemplVersion is the version of the templ generator. The ssr
	// shape writes its pages as .templ files, and the generator writes the
	// Go file of each one.
	DefaultTemplVersion = "v0.3.1020"
)

// validName reports a name that a module path can carry.
func validName(name string) bool {
	if name == "" {
		return false
	}
	for _, r := range name {
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '-' && r != '_' {
			return false
		}
	}
	return true
}

// ensureEmpty proves that the directory holds no file.
func ensureEmpty(dir string) error {
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return faultOf(fmt.Sprintf("the directory %s does not open", dir),
				"Give the process the right to write the parent directory")
		}
		return nil
	}
	if err != nil {
		return faultOf(fmt.Sprintf("the directory %s does not read", dir),
			"Give the process the right to read the directory")
	}
	if len(entries) > 0 {
		return faultOf(fmt.Sprintf("the directory %s already holds files", dir),
			"Run the command in an empty directory, or give the application another name")
	}
	return nil
}

// render writes one template set.
func render(set, dir string, values data) ([]string, error) {
	root := path.Join("templates", set)
	var written []string
	err := fs.WalkDir(templates, root, func(name string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		body, readErr := templates.ReadFile(name)
		if readErr != nil {
			return readErr
		}
		target := filepath.Join(dir, filepath.FromSlash(targetName(strings.TrimPrefix(name, root+"/"))))
		out, renderErr := execute(name, string(body), values)
		if renderErr != nil {
			return renderErr
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return faultOf(fmt.Sprintf("the directory %s does not open", filepath.Dir(target)),
				"Give the process the right to write the application directory")
		}
		if err := os.WriteFile(target, out, mode(target)); err != nil {
			return faultOf(fmt.Sprintf("the file %s does not write", target),
				"Give the process the right to write the application directory")
		}
		written = append(written, target)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return written, nil
}

// targetName turns a template name into the name of the written file. A
// template carries no leading dot, because go:embed skips such a file, so the
// scaffolder puts the dot back.
func targetName(name string) string {
	name = strings.TrimSuffix(name, ".tmpl")
	switch {
	case name == "gitignore":
		return ".gitignore"
	case name == "env.example":
		return ".env.example"
	case strings.HasPrefix(name, "claude-skills/"):
		// The skills follow the Agent Skills format: one directory for each
		// skill, and a SKILL.md with a name and a description in its
		// frontmatter. A coding agent reads .claude/skills of a project
		// without a setting. See the SDD, S16 part B.
		return ".claude/skills/" + strings.TrimPrefix(name, "claude-skills/")
	default:
		return name
	}
}

// mode returns the file mode of a written file.
func mode(target string) os.FileMode {
	if strings.HasSuffix(target, ".sh") {
		return 0o755
	}
	return 0o644
}

// executeSlice renders one slice template.
func executeSlice(name, body string, values sliceData) ([]byte, error) {
	t, err := template.New(name).Delims("[[", "]]").Parse(body)
	if err != nil {
		return nil, faultOf(fmt.Sprintf("the template %s does not parse: %v", name, err),
			"Report the fault, because a template of Avero must always parse")
	}
	var out strings.Builder
	if err := t.Execute(&out, values); err != nil {
		return nil, faultOf(fmt.Sprintf("the template %s does not render: %v", name, err),
			"Report the fault, because a template of Avero must always render")
	}
	return []byte(out.String()), nil
}

// execute renders one template.
//
// The delimiters are square brackets, because a Go source file holds a route
// pattern such as /posts/{id}, and a brace delimiter would read it as an
// action.
func execute(name, body string, values data) ([]byte, error) {
	t, err := template.New(path.Base(name)).Delims("[[", "]]").Parse(body)
	if err != nil {
		return nil, faultOf(fmt.Sprintf("the template %s does not parse: %v", name, err),
			"Report the fault, because a template of Avero must always parse")
	}
	var out strings.Builder
	if err := t.Execute(&out, values); err != nil {
		return nil, faultOf(fmt.Sprintf("the template %s does not render: %v", name, err),
			"Report the fault, because a template of Avero must always render")
	}
	return []byte(out.String()), nil
}

// title returns the name with the first letter in upper case and each dash as
// a space.
func title(name string) string {
	words := strings.FieldsFunc(name, func(r rune) bool { return r == '-' || r == '_' })
	for i, w := range words {
		if w == "" {
			continue
		}
		words[i] = strings.ToUpper(w[:1]) + w[1:]
	}
	return strings.Join(words, " ")
}
