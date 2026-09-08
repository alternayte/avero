package assets_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/alternayte/avero/assets"
)

func TestInitWritesThePackageFileOneTime(t *testing.T) {
	dir := t.TempDir()
	wrote, err := assets.Init(dir, "blog")
	if err != nil {
		t.Fatalf("Init returned %v, want nil", err)
	}
	if !wrote {
		t.Fatal("Init wrote no file")
	}
	body, err := os.ReadFile(filepath.Join(dir, assets.PackageName))
	if err != nil {
		t.Fatalf("ReadFile returned %v", err)
	}
	var doc map[string]any
	if err := json.Unmarshal(body, &doc); err != nil {
		t.Fatalf("the package file does not parse: %v", err)
	}
	if doc["name"] != "blog" || doc["type"] != "module" {
		t.Fatalf("the package file holds %v", doc)
	}

	again, err := assets.Init(dir, "blog")
	if err != nil {
		t.Fatalf("Init returned %v, want nil", err)
	}
	if again {
		t.Fatal("Init wrote the file a second time")
	}
}
