package assets

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

// PackageName is the file that tier 1 adds. esbuild then resolves a bare
// specifier against node_modules.
const PackageName = "package.json"

// Init writes the package.json of tier 1. `avero assets init` calls it.
//
// It reports whether it wrote the file. A file that already exists stays as it
// is, so the command is idempotent and never loses a dependency that a person
// added. The build command stays `avero build`.
func Init(dir, name string) (bool, error) {
	file := filepath.Join(dir, PackageName)
	if _, err := os.Stat(file); err == nil {
		return false, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return false, fault(PackageName, "the package file does not open",
			"Give the process the right to read the application directory")
	}
	if name == "" {
		name = filepath.Base(dir)
	}
	body, err := json.MarshalIndent(map[string]any{
		"name":            name,
		"private":         true,
		"type":            "module",
		"devDependencies": map[string]string{},
	}, "", "  ")
	if err != nil {
		return false, fault(PackageName, "the package file does not encode",
			"Report the fault, because the document always encodes")
	}
	if err := os.WriteFile(file, append(body, '\n'), 0o644); err != nil {
		return false, fault(PackageName, "the package file does not write",
			"Give the process the right to write the application directory")
	}
	return true, nil
}
