package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/alternayte/avero/assets"
	"github.com/alternayte/avero/internal/cli/uifiles"
)

// BasecoatStyles names the eight styles that Basecoat publishes. The first one
// is the default, and dist/basecoat.css imports it.
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
		if rest[i] == "--style" && i+1 < len(rest) {
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
// empty follows puts the line first. A file that holds the line already does
// not change, so a second run gives one result.
func addLine(dir, name, line, follows string) error {
	file := filepath.Join(dir, filepath.FromSlash(name))
	body, err := os.ReadFile(file)
	if err != nil {
		return fmt.Errorf("avero ui: %s does not open: %w\n  → Run the command in the root of an application that `avero new` wrote", name, err)
	}
	text := string(body)
	if strings.Contains(text, line) {
		return nil
	}
	lines := strings.Split(text, "\n")
	at := 0
	if follows != "" {
		for i, one := range lines {
			if strings.TrimSpace(one) == follows {
				at = i + 1
				break
			}
		}
	}
	out := append([]string{}, lines[:at]...)
	out = append(out, line)
	out = append(out, lines[at:]...)
	if err := os.WriteFile(file, []byte(strings.Join(out, "\n")), 0o644); err != nil {
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
