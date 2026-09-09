package assets_test

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alternayte/avero/assets"
)

// tarball returns a gzip tarball that holds each named file.
func tarball(t *testing.T, files map[string]string) string {
	t.Helper()
	var buf bytes.Buffer
	zip := gzip.NewWriter(&buf)
	out := tar.NewWriter(zip)
	for name, body := range files {
		if err := out.WriteHeader(&tar.Header{
			Name: name, Mode: 0o644, Size: int64(len(body)), Typeflag: tar.TypeReg,
		}); err != nil {
			t.Fatalf("WriteHeader returned %v", err)
		}
		if _, err := out.Write([]byte(body)); err != nil {
			t.Fatalf("Write returned %v", err)
		}
	}
	if err := out.Close(); err != nil {
		t.Fatalf("Close returned %v", err)
	}
	if err := zip.Close(); err != nil {
		t.Fatalf("Close returned %v", err)
	}
	return buf.String()
}

const packageURL = "https://registry.example.com/basecoat-css-1.0.2.tgz"

// basecoatTar returns a small package of the shape that npm publishes.
func basecoatTar(t *testing.T) string {
	t.Helper()
	return tarball(t, map[string]string{
		"package/package.json":       "{\"name\":\"basecoat-css\"}\n",
		"package/dist/basecoat.css":  "@import \"./basecoat-vega.css\";\n",
		"package/dist/js/all.min.js": "(() => {})();\n",
		"package/README.md":          "read me\n",
	})
}

func TestPinPackageWritesTheTreeAndTheLock(t *testing.T) {
	dir := t.TempDir()
	f := &fetcher{bodies: map[string]string{packageURL: basecoatTar(t)}}

	if err := assets.PinPackage(context.Background(), assets.PinConfig{Dir: dir, Fetch: f}, "basecoat", packageURL); err != nil {
		t.Fatalf("PinPackage returned %v, want nil", err)
	}

	root := filepath.Join(dir, "assets", "vendor", "basecoat")
	if body, err := os.ReadFile(filepath.Join(root, "basecoat.css")); err != nil || string(body) != "@import \"./basecoat-vega.css\";\n" {
		t.Fatalf("the stylesheet is %q and the error is %v", body, err)
	}
	if _, err := os.Stat(filepath.Join(root, "js", "all.min.js")); err != nil {
		t.Fatalf("the script is absent: %v", err)
	}
	// The pin writes the dist directory and nothing else.
	if _, err := os.Stat(filepath.Join(root, "package.json")); err == nil {
		t.Fatal("the pin wrote a file from outside dist")
	}

	lock, err := assets.LoadLock(dir)
	if err != nil {
		t.Fatalf("LoadLock returned %v, want nil", err)
	}
	pin, ok := lock.Package("basecoat")
	if !ok {
		t.Fatal("the lock holds no package")
	}
	if pin.URL != packageURL || pin.SHA256 == "" {
		t.Fatalf("the pin holds %+v", pin)
	}
	want := []string{"basecoat.css", "js/all.min.js"}
	if len(pin.Files) != len(want) || pin.Files[0] != want[0] || pin.Files[1] != want[1] {
		t.Fatalf("the pin records the files %v, want %v", pin.Files, want)
	}
}

func TestPinPackageIsIdempotent(t *testing.T) {
	dir := t.TempDir()
	f := &fetcher{bodies: map[string]string{packageURL: basecoatTar(t)}}
	cfg := assets.PinConfig{Dir: dir, Fetch: f}

	if err := assets.PinPackage(context.Background(), cfg, "basecoat", packageURL); err != nil {
		t.Fatalf("the first pin returned %v", err)
	}
	if err := assets.PinPackage(context.Background(), cfg, "basecoat", packageURL); err != nil {
		t.Fatalf("the second pin returned %v", err)
	}
	if f.calls != 1 {
		t.Fatalf("the fetcher ran %d times, want 1", f.calls)
	}
}

func TestPinPackageRepairsAnAbsentFile(t *testing.T) {
	dir := t.TempDir()
	f := &fetcher{bodies: map[string]string{packageURL: basecoatTar(t)}}
	cfg := assets.PinConfig{Dir: dir, Fetch: f}

	if err := assets.PinPackage(context.Background(), cfg, "basecoat", packageURL); err != nil {
		t.Fatalf("the first pin returned %v", err)
	}
	if err := os.Remove(filepath.Join(dir, "assets", "vendor", "basecoat", "js", "all.min.js")); err != nil {
		t.Fatalf("Remove returned %v", err)
	}
	if err := assets.PinPackage(context.Background(), cfg, "basecoat", packageURL); err != nil {
		t.Fatalf("the second pin returned %v", err)
	}
	if f.calls != 2 {
		t.Fatalf("the fetcher ran %d times, want 2", f.calls)
	}
	if _, err := os.Stat(filepath.Join(dir, "assets", "vendor", "basecoat", "js", "all.min.js")); err != nil {
		t.Fatalf("the pin did not write the absent file: %v", err)
	}
}

func TestPinPackageFailsOnAHashMismatch(t *testing.T) {
	dir := t.TempDir()
	f := &fetcher{bodies: map[string]string{packageURL: basecoatTar(t)}}
	cfg := assets.PinConfig{Dir: dir, Fetch: f}

	if err := assets.PinPackage(context.Background(), cfg, "basecoat", packageURL); err != nil {
		t.Fatalf("the first pin returned %v", err)
	}
	// The address answers other bytes now.
	f.bodies[packageURL] = tarball(t, map[string]string{"package/dist/basecoat.css": "body{}\n"})
	if err := os.RemoveAll(filepath.Join(dir, "assets", "vendor", "basecoat")); err != nil {
		t.Fatalf("RemoveAll returned %v", err)
	}

	err := assets.PinPackage(context.Background(), cfg, "basecoat", packageURL)
	if err == nil {
		t.Fatal("PinPackage returned nil for a body that changed")
	}
	if !strings.Contains(err.Error(), "avero.lock") {
		t.Fatalf("the fault does not name the lock: %v", err)
	}
}

func TestPinPackageRefusesAPathThatLeavesTheDirectory(t *testing.T) {
	dir := t.TempDir()
	f := &fetcher{bodies: map[string]string{packageURL: tarball(t, map[string]string{
		"package/dist/basecoat.css":     "@import \"./basecoat-vega.css\";\n",
		"package/dist/../../escape.css": "body{}\n",
	})}}

	if err := assets.PinPackage(context.Background(), assets.PinConfig{Dir: dir, Fetch: f}, "basecoat", packageURL); err != nil {
		t.Fatalf("PinPackage returned %v, want nil for a tarball with one valid file", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "assets", "vendor", "basecoat", "basecoat.css")); err != nil {
		t.Fatalf("the pin did not write the valid file: %v", err)
	}
	if err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && filepath.Base(path) == "escape.css" {
			t.Fatalf("the pin wrote the escaping file at %s", path)
		}
		return nil
	}); err != nil {
		t.Fatalf("Walk returned %v", err)
	}
}
