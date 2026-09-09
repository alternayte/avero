// Package ui holds the shell of the single page application. The front end
// lives in assets/js. This package writes the document that loads it.
package ui

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/alternayte/avero"
	"github.com/alternayte/avero/view"
)

// manifest resolves the name of an asset to its hashed path. wire.go sets it
// one time at start.
var manifest *avero.Manifest

// SetManifest records the asset manifest of the application.
func SetManifest(m *avero.Manifest) { manifest = m }

// Asset returns the path of a built asset.
func Asset(name string) string { return manifest.Asset(name) }

// Shell renders the document that loads the front end.
func Shell() avero.ViewComponent {
	return view.Func(func(_ context.Context, w io.Writer) error {
		var b strings.Builder
		b.WriteString("<!doctype html>\n<html lang=\"en\">\n<head>\n")
		b.WriteString("<meta charset=\"utf-8\">\n")
		b.WriteString("<meta name=\"viewport\" content=\"width=device-width, initial-scale=1\">\n")
		b.WriteString("<title>Board</title>\n")
		fmt.Fprintf(&b, "<link rel=\"stylesheet\" href=%q>\n", Asset("app.css"))
		fmt.Fprintf(&b, "<script type=\"module\" src=%q></script>\n", Asset("app.js"))
		b.WriteString("</head>\n<body>\n<div id=\"app\"></div>\n</body>\n</html>\n")
		_, err := io.WriteString(w, b.String())
		return err
	})
}
