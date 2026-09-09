package scaffold

import (
	"encoding/json"
	"fmt"
	"os"
	"path"
	"path/filepath"

	"github.com/alternayte/avero/assets"
)

// The starter files of the output directory. `avero build` replaces them with
// the bundle of esbuild and the stylesheet of Tailwind.
const (
	starterCSS = "/* The starter stylesheet. `avero build` replaces it with the\n" +
		"   output of Tailwind. */\nbody { font-family: system-ui, sans-serif; }\n"
	starterJS = "// The starter script. `avero build` replaces it with the bundle of\n" +
		"// esbuild.\n"
	// starterIndex stands until Vite writes the index document of the front
	// end.
	starterIndex = "<!doctype html>\n<html lang=\"en\">\n<head>\n" +
		"<meta charset=\"utf-8\">\n<title>The front end is not built</title>\n" +
		"</head>\n<body>\n<div id=\"root\"></div>\n" +
		"<p>Run <code>avero build</code>, which builds the front end into this directory.</p>\n" +
		"</body>\n</html>\n"
)

// WriteStarterAssets writes the output directory of the assets of an
// application that already exists.
//
// A clone of a repository holds no built asset, because an application ignores
// its output directory. The command that runs an example calls this function,
// so the embedded file system holds a document before the first build.
func WriteStarterAssets(dir, shape string) ([]string, error) { return writeStarterAssets(dir, shape) }

// writeStarterIndex writes the index document that stands until Vite writes
// its own.
func writeStarterIndex(dir string) ([]string, error) {
	out := filepath.Join(dir, filepath.FromSlash(assets.OutDir))
	if err := os.MkdirAll(out, 0o755); err != nil {
		return nil, faultOf(fmt.Sprintf("the directory %s does not open", out),
			"Give the process the right to write the application directory")
	}
	name := filepath.Join(out, assets.IndexFile)
	if err := os.WriteFile(name, []byte(starterIndex), 0o644); err != nil {
		return nil, faultOf(fmt.Sprintf("the file %s does not write", name),
			"Give the process the right to write the application directory")
	}
	return []string{name}, nil
}

// writeStarterAssets writes the output directory of the assets.
//
// The application embeds that directory, so it must hold a manifest before the
// first build. A person therefore runs the application at once, and
// `avero build` replaces every file.
func writeStarterAssets(dir, shape string) ([]string, error) {
	if shape == ShapeSPA {
		// Vite writes the index document of the front end. The starter one
		// states the repair, so the application builds and runs before the
		// first build of the front end.
		return writeStarterIndex(dir)
	}
	out := filepath.Join(dir, filepath.FromSlash(assets.OutDir))
	if err := os.MkdirAll(filepath.Join(out, "css"), 0o755); err != nil {
		return nil, faultOf(fmt.Sprintf("the directory %s does not open", out),
			"Give the process the right to write the application directory")
	}
	if err := os.MkdirAll(filepath.Join(out, "js"), 0o755); err != nil {
		return nil, faultOf(fmt.Sprintf("the directory %s does not open", out),
			"Give the process the right to write the application directory")
	}

	m := assets.NewManifest(assets.DefaultBase)
	var written []string
	for _, starter := range []struct{ name, body string }{
		{"css/app.css", starterCSS},
		{"js/app.js", starterJS},
	} {
		hash := assets.Hash([]byte(starter.body))
		file := assets.HashedName(starter.name, hash)
		target := filepath.Join(out, filepath.FromSlash(file))
		if err := os.WriteFile(target, []byte(starter.body), 0o644); err != nil {
			return nil, faultOf(fmt.Sprintf("the file %s does not write", target),
				"Give the process the right to write the application directory")
		}
		entry := assets.Entry{
			Source: starter.name, File: file, Hash: hash,
			Size: int64(len(starter.body)), Integrity: assets.Integrity([]byte(starter.body)),
		}
		m.Set(entry)
		entry.Source = path.Base(starter.name)
		m.Set(entry)
		written = append(written, target)
	}

	body, err := json.Marshal(m)
	if err != nil {
		return nil, faultOf("the asset manifest does not encode",
			"Report the fault, because a manifest always encodes")
	}
	name := filepath.Join(out, assets.ManifestName)
	if err := os.WriteFile(name, append(body, '\n'), 0o644); err != nil {
		return nil, faultOf(fmt.Sprintf("the file %s does not write", name),
			"Give the process the right to write the application directory")
	}
	return append(written, name), nil
}
