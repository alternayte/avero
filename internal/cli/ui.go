package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/alternayte/avero/assets"
	"github.com/alternayte/avero/internal/cli/uifiles"
	"github.com/alternayte/avero/scaffold"
)

// BasecoatStyles names the eight styles that Basecoat publishes. The first one
// is the default. dist/basecoat.css imports the default style.
var BasecoatStyles = []string{"vega", "nova", "maia", "lyra", "mira", "luma", "sera", "rhea"}

// uiFetch reads the package of a library. A nil value reads over HTTP. A test
// replaces it, so the gate needs no network.
var uiFetch assets.Fetcher

// SetUIFetcher replaces the fetcher of `avero ui`. A test calls it.
func SetUIFetcher(f assets.Fetcher) { uiFetch = f }

// BasecoatURL returns the address of one release of Basecoat.
func BasecoatURL(version string) string {
	return fmt.Sprintf("https://registry.npmjs.org/basecoat-css/-/basecoat-css-%s.tgz", version)
}

// runUI adds a component library to an application.
func runUI(ctx context.Context, s Streams, args []string) int {
	if len(args) < 2 || args[0] != "add" {
		return failf(s, "avero ui: the form is `avero ui add <library> [--style <name>]`\n  → Run `avero ui add basecoat`")
	}
	if args[1] != "basecoat" {
		return failf(s, "avero ui: the library %q is unknown\n  → The known library is basecoat. Run `avero ui add basecoat`", args[1])
	}
	style := BasecoatStyles[0]
	rest := args[2:]
	for i := 0; i < len(rest); i++ {
		if rest[i] == "--style" {
			if i+1 >= len(rest) {
				return failf(s, "avero ui: the flag --style names no value\n  → Run `avero ui add basecoat --style %s`", BasecoatStyles[0])
			}
			style = rest[i+1]
			i++
			continue
		}
		return failf(s, "avero ui: the argument %q is unknown\n  → The form is `avero ui add basecoat [--style %s]`", rest[i], BasecoatStyles[0])
	}
	if !known(style) {
		return failf(s, "avero ui: the style %q is unknown\n  → Name one of %s", style, strings.Join(BasecoatStyles, ", "))
	}

	dir := dirOf(s)
	project, err := LoadProject(dir)
	if err != nil {
		return fail(s, err)
	}
	if project.Shape != "ssr" {
		return failf(s, "avero ui: the shape of this application is %q and it renders no page\n  → Add Basecoat to an application of the ssr shape", project.Shape)
	}

	cfg := assets.PinConfig{Dir: dir, Fetch: uiFetch}
	if err := assets.PinPackage(ctx, cfg, "basecoat", BasecoatURL(assets.BasecoatVersion)); err != nil {
		return fail(s, err)
	}
	_, _ = fmt.Fprintf(s.Out, "pinned basecoat %s\n", assets.BasecoatVersion)

	if err := addLine(dir, "assets/css/app.css",
		fmt.Sprintf("@import \"../vendor/basecoat/basecoat-%s.css\";", style),
		"@import \"tailwindcss\";"); err != nil {
		return fail(s, err)
	}
	if err := addLine(dir, "assets/js/app.js",
		"import \"../vendor/basecoat/js/all.min.js\";", ""); err != nil {
		return fail(s, err)
	}
	if err := writeToaster(dir, s); err != nil {
		return fail(s, err)
	}
	if err := writeToasterTest(dir, s); err != nil {
		return fail(s, err)
	}
	if err := patchLayout(dir, s, project.Name); err != nil {
		return fail(s, err)
	}
	_, _ = fmt.Fprintf(s.Out, "\nRun `avero dev` and read the page.\n")
	return 0
}

// known reports whether the name is a style of Basecoat.
func known(style string) bool {
	for _, name := range BasecoatStyles {
		if name == style {
			return true
		}
	}
	return false
}

// addLine puts one line into a file, after the line that follows names. An
// empty follows puts the line first.
//
// The command compares one line against one line, with the spaces of each end
// removed. A file that holds the line already, in any spacing, does not
// change, so a second run gives one result.
//
// The command keeps the line end of the file. It reads CRLF when the file
// holds one CRLF pair, and LF otherwise.
//
// The command fails when follows names a line that the file does not hold,
// because the command would then guess where the line belongs.
func addLine(dir, name, line, follows string) error {
	file := filepath.Join(dir, filepath.FromSlash(name))
	body, err := os.ReadFile(file)
	if err != nil {
		return fmt.Errorf("avero ui: %s does not open: %w\n  → Run the command in the root of an application that `avero new` wrote", name, err)
	}
	eol := "\n"
	if strings.Contains(string(body), "\r\n") {
		eol = "\r\n"
	}
	lines := strings.Split(string(body), eol)
	want := strings.TrimSpace(line)
	for _, one := range lines {
		if strings.TrimSpace(one) == want {
			return nil
		}
	}
	at := 0
	if follows != "" {
		found := false
		for i, one := range lines {
			if strings.TrimSpace(one) == strings.TrimSpace(follows) {
				at = i + 1
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("avero ui: %s states no %q line\n  → Add the line by hand, or restore the entry point that `avero new` wrote", name, follows)
		}
	}
	out := append([]string{}, lines[:at]...)
	out = append(out, line)
	out = append(out, lines[at:]...)
	if err := os.WriteFile(file, []byte(strings.Join(out, eol)), 0o644); err != nil {
		return fmt.Errorf("avero ui: %s does not write: %w\n  → Give the process the right to write the application directory", name, err)
	}
	return nil
}

// writeToaster writes the toaster component. A file that exists does not
// change, because a person owns it after the first run.
func writeToaster(dir string, s Streams) error {
	file := filepath.Join(dir, "internal", "ui", "toaster.templ")
	if _, err := os.Stat(file); err == nil {
		_, _ = fmt.Fprintln(s.Out, "internal/ui/toaster.templ exists and does not change")
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
		return fmt.Errorf("avero ui: internal/ui does not open: %w\n  → Give the process the right to write the application directory", err)
	}
	if err := os.WriteFile(file, uifiles.Toaster(), 0o644); err != nil {
		return fmt.Errorf("avero ui: internal/ui/toaster.templ does not write: %w\n  → Give the process the right to write the application directory", err)
	}
	_, _ = fmt.Fprintln(s.Out, "wrote internal/ui/toaster.templ")
	return nil
}

// writeToasterTest writes the proof of the toaster component. A file that
// exists does not change, because a person owns it after the first run.
func writeToasterTest(dir string, s Streams) error {
	file := filepath.Join(dir, "toaster_test.go")
	if _, err := os.Stat(file); err == nil {
		_, _ = fmt.Fprintln(s.Out, "toaster_test.go exists and does not change")
		return nil
	}
	if err := os.WriteFile(file, uifiles.ToasterTest(), 0o644); err != nil {
		return fmt.Errorf("avero ui: toaster_test.go does not write: %w\n  → Give the process the right to write the application directory", err)
	}
	_, _ = fmt.Fprintln(s.Out, "wrote toaster_test.go")
	return nil
}

// scaffoldToasts is the block that the ssr layout of the scaffold defines. The
// command removes it, because the toaster component defines the same name.
const scaffoldToasts = "// Toasts renders the messages of this response."

// patchLayout moves the call of Toasts to the end of the body and removes the
// definition that the scaffold wrote.
//
// The command compares the file on disk with the layout that the scaffold
// renders for this application. It patches an exact match. It changes
// nothing, and prints nothing, when the file already carries the patch,
// because a second run must give one result. It changes nothing, and prints
// the two lines, when the file matches neither state, because a person who
// changed the layout owns it. See the design, D2.
func patchLayout(dir string, s Streams, name string) error {
	file := filepath.Join(dir, "internal", "ui", "layout.templ")
	got, err := os.ReadFile(file)
	if err != nil {
		return nil
	}

	want, err := scaffold.Layout(name)
	if err != nil {
		return err
	}
	done := patchOf(want)

	switch {
	case string(got) == string(want):
		if err := os.WriteFile(file, done, 0o644); err != nil {
			return fmt.Errorf("avero ui: internal/ui/layout.templ does not write: %w\n  → Give the process the right to write the application directory", err)
		}
		_, _ = fmt.Fprintln(s.Out, "patched internal/ui/layout.templ")
	case string(got) == string(done):
		// The layout already carries the patch, so the command changes
		// nothing and prints nothing.
	default:
		printLayoutLines(s)
	}
	return nil
}

// patchOf renders the patch that patchLayout applies to the layout that the
// scaffold writes. It moves the call of Toasts to the end of the body, with
// the indentation of the other children of the body element, and it removes
// the definition of Toasts, because the toaster component defines that name.
//
// The function reads the exact body of the scaffold layout, so it never
// removes a call that a person put in another place.
func patchOf(scaffoldLayout []byte) []byte {
	lines := strings.Split(string(scaffoldLayout), "\n")

	// Remove the call from its place inside main.
	kept := lines[:0]
	for _, line := range lines {
		if strings.TrimSpace(line) != "@Toasts()" {
			kept = append(kept, line)
		}
	}
	lines = kept

	// Put the call before the end of the body, with the indentation of the
	// other children of the body element, because the toaster is a fixed
	// element of the page and not of the content.
	for i, line := range lines {
		if !strings.Contains(line, "</body>") {
			continue
		}
		indent := line[:len(line)-len(strings.TrimLeft(line, "\t"))]
		call := indent + "\t@Toasts()"
		lines = append(lines[:i:i], append([]string{call}, lines[i:]...)...)
		break
	}
	text := strings.Join(lines, "\n")

	// Remove the definition of the scaffold, which reaches to the end of the
	// file.
	if cut := strings.Index(text, scaffoldToasts); cut >= 0 {
		text = strings.TrimRight(text[:cut], "\n") + "\n"
	}
	return []byte(text)
}

// printLayoutLines states the change that a person makes by hand.
func printLayoutLines(s Streams) {
	_, _ = fmt.Fprint(s.Out, `
internal/ui/layout.templ does not match the scaffold, so it did not change.
Make two changes by hand:

  1. Move @Toasts() out of <main> and put it before </body>.
  2. Delete the templ Toasts() block, because internal/ui/toaster.templ
     defines that name now.
`)
}
