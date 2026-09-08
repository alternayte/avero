package assets_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/alternayte/avero/assets"
)

// manifest returns a manifest with one CSS file and one JS file.
func manifest() *assets.Manifest {
	m := assets.NewManifest("/assets/")
	m.Set(assets.Entry{Source: "app.css", File: "app.a1b2c3d4e5f6.css", Hash: "a1b2c3d4e5f6", Size: 3})
	m.Set(assets.Entry{Source: "app.js", File: "app.0123456789ab.js", Hash: "0123456789ab", Size: 2})
	return m
}

func TestAssetResolvesToTheHashedPath(t *testing.T) {
	if got := manifest().Asset("app.css"); got != "/assets/app.a1b2c3d4e5f6.css" {
		t.Fatalf("Asset = %q", got)
	}
}

func TestAssetReturnsTheNameOfAnAbsentFile(t *testing.T) {
	// A missing entry must not break the page. The name resolves to the
	// unhashed path, and Err names the fault.
	m := manifest()
	if got := m.Asset("absent.css"); got != "/assets/absent.css" {
		t.Fatalf("Asset = %q", got)
	}
}

func TestAChangedFileProducesADifferentHash(t *testing.T) {
	first := assets.Hash([]byte("body{}"))
	second := assets.Hash([]byte("body{color:red}"))
	if first == second {
		t.Fatal("two contents share one hash")
	}
	if len(first) != 12 {
		t.Fatalf("the hash holds %d characters, want 12", len(first))
	}
	if got := assets.HashedName("css/app.css", first); got != "css/app."+first+".css" {
		t.Fatalf("HashedName = %q", got)
	}
}

func TestTheManifestRoundTrips(t *testing.T) {
	b, err := json.Marshal(manifest())
	if err != nil {
		t.Fatalf("Marshal returned an error: %v", err)
	}
	var back assets.Manifest
	if err := json.Unmarshal(b, &back); err != nil {
		t.Fatalf("Unmarshal returned an error: %v", err)
	}
	if got := back.Asset("app.js"); got != "/assets/app.0123456789ab.js" {
		t.Fatalf("Asset = %q", got)
	}
	if !strings.Contains(string(b), assets.ManifestSchemaID) {
		t.Fatalf("the manifest carries no schema: %s", b)
	}
}

func TestLoadManifestReadsTheFileSystem(t *testing.T) {
	b, err := json.Marshal(manifest())
	if err != nil {
		t.Fatalf("Marshal returned an error: %v", err)
	}
	fsys := fstest.MapFS{"dist/manifest.json": &fstest.MapFile{Data: b}}
	m, err := assets.LoadManifest(fsys, "dist/manifest.json")
	if err != nil {
		t.Fatalf("LoadManifest returned %v, want nil", err)
	}
	if got := m.Asset("app.css"); got != "/assets/app.a1b2c3d4e5f6.css" {
		t.Fatalf("Asset = %q", got)
	}
}

func TestLoadManifestNamesTheRepair(t *testing.T) {
	_, err := assets.LoadManifest(fstest.MapFS{}, "dist/manifest.json")
	if err == nil {
		t.Fatal("LoadManifest returned nil, want a fault")
	}
	if !strings.Contains(err.Error(), "avero build") {
		t.Fatalf("the fault states no repair: %v", err)
	}
}

func TestTheHandlerServesTheEmbeddedFileSystem(t *testing.T) {
	fsys := fstest.MapFS{
		"app.a1b2c3d4e5f6.css": &fstest.MapFile{Data: []byte("body{}")},
		"app.0123456789ab.js":  &fstest.MapFile{Data: []byte("1")},
		"secret.txt":           &fstest.MapFile{Data: []byte("no")},
	}
	h := assets.Handler(fsys, manifest())

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/app.a1b2c3d4e5f6.css", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if got := rec.Header().Get("Cache-Control"); got != assets.CacheControl {
		t.Fatalf("Cache-Control = %q, want %q", got, assets.CacheControl)
	}
	if got := rec.Header().Get("Content-Type"); !strings.HasPrefix(got, "text/css") {
		t.Fatalf("Content-Type = %q", got)
	}

	other := httptest.NewRecorder()
	h.ServeHTTP(other, httptest.NewRequest(http.MethodGet, "/secret.txt", nil))
	if other.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404 for a file outside the manifest", other.Code)
	}
}

func TestTheHandlerAnswersOnlyASafeMethod(t *testing.T) {
	h := assets.Handler(fstest.MapFS{"app.a1b2c3d4e5f6.css": &fstest.MapFile{Data: []byte("body{}")}}, manifest())
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/app.a1b2c3d4e5f6.css", nil))
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want 405", rec.Code)
	}
}
