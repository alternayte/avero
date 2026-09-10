package avero_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"

	avero "github.com/alternayte/avero"
	"github.com/alternayte/avero/assets"
)

// MountAssets reads the manifest and serves the built file at /assets/.
func TestMountAssets(t *testing.T) {
	// Create a proper manifest structure.
	manifest := assets.NewManifest("/assets/")
	manifest.Set(assets.Entry{
		Source:    "css/app.css",
		File:      "css/app-abc123.css",
		Hash:      "abc123",
		Size:      6,
		Integrity: "",
	})
	manifestJSON, err := json.Marshal(manifest)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	dist := fstest.MapFS{
		"assets/dist/manifest.json":      &fstest.MapFile{Data: manifestJSON},
		"assets/dist/css/app-abc123.css": &fstest.MapFile{Data: []byte("body{}")},
	}

	r := avero.NewRouter()
	mountedManifest, err := avero.MountAssets(r, dist, "assets/dist")
	if err != nil {
		t.Fatalf("MountAssets failed: %v", err)
	}
	if got := mountedManifest.Asset("css/app.css"); got != "/assets/css/app-abc123.css" {
		t.Fatalf("the manifest holds %q", got)
	}

	handler, err := r.Handler()
	if err != nil {
		t.Fatalf("the handler does not build: %v", err)
	}
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/assets/css/app-abc123.css", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("the asset answers %d", rec.Code)
	}
}

// An absent manifest names the fault and returns no manifest.
func TestMountAssetsWithNoManifest(t *testing.T) {
	if _, err := avero.MountAssets(avero.NewRouter(), fstest.MapFS{}, "assets/dist"); err == nil {
		t.Fatal("MountAssets returned no error for an absent manifest")
	}
}
