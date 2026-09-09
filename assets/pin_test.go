package assets_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/alternayte/avero/assets"
)

// fetcher returns a body for each URL and counts the calls.
type fetcher struct {
	bodies map[string]string
	calls  int
}

func (f *fetcher) Fetch(_ context.Context, url string) ([]byte, error) {
	f.calls++
	body, ok := f.bodies[url]
	if !ok {
		return nil, os.ErrNotExist
	}
	return []byte(body), nil
}

const nanoURL = "https://cdn.example.com/nanostores@1.0.0/index.js"

func TestPinWritesTheVendorFileAndTheLock(t *testing.T) {
	dir := t.TempDir()
	f := &fetcher{bodies: map[string]string{nanoURL: "export const atom = () => 1;\n"}}

	if err := assets.PinJS(context.Background(), assets.PinConfig{Dir: dir, Fetch: f}, "nanostores", nanoURL); err != nil {
		t.Fatalf("PinJS returned %v, want nil", err)
	}
	body, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(assets.VendorDir), "nanostores.js"))
	if err != nil {
		t.Fatalf("the vendor file is absent: %v", err)
	}
	sum := sha256.Sum256(body)

	lock, err := assets.LoadLock(dir)
	if err != nil {
		t.Fatalf("LoadLock returned %v, want nil", err)
	}
	pin, ok := lock.JS("nanostores")
	if !ok {
		t.Fatalf("the lock holds no pin, it holds %v", lock.Names())
	}
	if pin.URL != nanoURL || pin.SHA256 != hex.EncodeToString(sum[:]) {
		t.Fatalf("the pin holds %+v", pin)
	}
}

func TestPinIsIdempotent(t *testing.T) {
	dir := t.TempDir()
	f := &fetcher{bodies: map[string]string{nanoURL: "export const atom = () => 1;\n"}}
	cfg := assets.PinConfig{Dir: dir, Fetch: f}

	for i := range 3 {
		if err := assets.PinJS(context.Background(), cfg, "nanostores", nanoURL); err != nil {
			t.Fatalf("call %d returned %v, want nil", i+1, err)
		}
	}
	if f.calls != 1 {
		t.Fatalf("the fetcher ran %d times, want 1", f.calls)
	}
}

func TestPinFailsOnAHashMismatch(t *testing.T) {
	dir := t.TempDir()
	f := &fetcher{bodies: map[string]string{nanoURL: "export const atom = () => 1;\n"}}
	cfg := assets.PinConfig{Dir: dir, Fetch: f}
	if err := assets.PinJS(context.Background(), cfg, "nanostores", nanoURL); err != nil {
		t.Fatalf("PinJS returned %v, want nil", err)
	}

	// The CDN answers with other bytes under the same URL, and the vendor
	// file is absent, so the pin fetches again.
	if err := os.Remove(filepath.Join(dir, filepath.FromSlash(assets.VendorDir), "nanostores.js")); err != nil {
		t.Fatalf("Remove returned %v", err)
	}
	f.bodies[nanoURL] = "export const atom = () => 2;\n"
	err := assets.PinJS(context.Background(), cfg, "nanostores", nanoURL)
	if err == nil {
		t.Fatal("PinJS returned nil, want a fault")
	}
	if !strings.Contains(err.Error(), "nanostores") || !strings.Contains(err.Error(), assets.LockName) {
		t.Fatalf("the fault does not name the package and the lock:\n%v", err)
	}
}

func TestPinRepairsAnAbsentVendorFile(t *testing.T) {
	dir := t.TempDir()
	f := &fetcher{bodies: map[string]string{nanoURL: "export const atom = () => 1;\n"}}
	cfg := assets.PinConfig{Dir: dir, Fetch: f}
	if err := assets.PinJS(context.Background(), cfg, "nanostores", nanoURL); err != nil {
		t.Fatalf("PinJS returned %v, want nil", err)
	}
	file := filepath.Join(dir, filepath.FromSlash(assets.VendorDir), "nanostores.js")
	if err := os.Remove(file); err != nil {
		t.Fatalf("Remove returned %v", err)
	}
	if err := assets.PinJS(context.Background(), cfg, "nanostores", nanoURL); err != nil {
		t.Fatalf("PinJS returned %v, want nil", err)
	}
	if _, err := os.Stat(file); err != nil {
		t.Fatalf("the vendor file did not return: %v", err)
	}
	if f.calls != 2 {
		t.Fatalf("the fetcher ran %d times, want 2", f.calls)
	}
}

func TestPinWithNoURLReadsTheLock(t *testing.T) {
	dir := t.TempDir()
	f := &fetcher{bodies: map[string]string{nanoURL: "export const atom = () => 1;\n"}}
	cfg := assets.PinConfig{Dir: dir, Fetch: f}
	if err := assets.PinJS(context.Background(), cfg, "nanostores", nanoURL); err != nil {
		t.Fatalf("PinJS returned %v, want nil", err)
	}
	if err := assets.PinJS(context.Background(), cfg, "nanostores", ""); err != nil {
		t.Fatalf("PinJS returned %v, want nil", err)
	}
	if err := assets.PinJS(context.Background(), cfg, "absent", ""); err == nil {
		t.Fatal("PinJS returned nil for a package that the lock does not hold")
	}
}

func TestTailwindDownloadsTheBinaryOneTime(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the test writes a shell script as the binary")
	}
	dir := t.TempDir()
	tw := &assets.Tailwind{Version: "4.1.0", Dir: dir}
	f := &fetcher{bodies: map[string]string{tw.URL(): "#!/bin/sh\nprintf 'h1{color:red}'\n"}}
	tw.Fetch = f

	first, err := tw.Ensure(context.Background())
	if err != nil {
		t.Fatalf("Ensure returned %v, want nil", err)
	}
	if !strings.Contains(first, filepath.FromSlash(".avero/bin")) {
		t.Fatalf("the binary lives at %q, want .avero/bin", first)
	}
	info, err := os.Stat(first)
	if err != nil {
		t.Fatalf("Stat returned %v", err)
	}
	if info.Mode().Perm()&0o100 == 0 {
		t.Fatalf("the mode is %v, want an executable file", info.Mode())
	}
	if _, err := tw.Ensure(context.Background()); err != nil {
		t.Fatalf("Ensure returned %v, want nil", err)
	}
	if f.calls != 1 {
		t.Fatalf("the fetcher ran %d times, want 1", f.calls)
	}

	lock, err := assets.LoadLock(dir)
	if err != nil {
		t.Fatalf("LoadLock returned %v, want nil", err)
	}
	if _, ok := lock.Bin("tailwindcss"); !ok {
		t.Fatal("the lock holds no pin of the binary")
	}
}

func TestTailwindCompilesTheStylesheet(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the test writes a shell script as the binary")
	}
	dir := t.TempDir()
	write(t, dir, "assets/css/app.css", "@import \"tailwindcss\";\n")
	tw := &assets.Tailwind{Version: "4.1.0", Dir: dir, Input: "assets/css/app.css"}
	// The script writes the file that -o names, as the real binary does.
	f := &fetcher{bodies: map[string]string{tw.URL(): "#!/bin/sh\nwhile [ $# -gt 0 ]; do if [ \"$1\" = \"-o\" ]; then printf 'h1{color:red}' > \"$2\"; fi; shift; done\n"}}
	tw.Fetch = f

	css, err := tw.Compile(context.Background())
	if err != nil {
		t.Fatalf("Compile returned %v, want nil", err)
	}
	if string(css) != "h1{color:red}" {
		t.Fatalf("Compile returned %q", css)
	}
}

func TestTheBuildTakesTheTailwindOutput(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the test writes a shell script as the binary")
	}
	dir := project(t)
	tw := &assets.Tailwind{Version: "4.1.0", Dir: dir, Input: "assets/css/app.css", Asset: "css/app.css"}
	tw.Fetch = &fetcher{bodies: map[string]string{tw.URL(): "#!/bin/sh\nwhile [ $# -gt 0 ]; do if [ \"$1\" = \"-o\" ]; then printf 'h1{color:blue}' > \"$2\"; fi; shift; done\n"}}

	cfg := assets.Config{Dir: dir, Entries: []string{"js/app.js"}, Tailwind: tw}
	m, err := assets.Build(context.Background(), cfg)
	if err != nil {
		t.Fatalf("Build returned %v, want nil", err)
	}
	entry, ok := m.Entry("css/app.css")
	if !ok {
		t.Fatalf("the manifest holds no stylesheet, it holds %v", m.Names())
	}
	body, err := os.ReadFile(filepath.Join(dir, "assets/dist", entry.File))
	if err != nil {
		t.Fatalf("ReadFile returned %v", err)
	}
	if string(body) != "h1{color:blue}" {
		t.Fatalf("the stylesheet holds %q", body)
	}
}

func TestResolveReadsTheModuleOfAPackage(t *testing.T) {
	answer := "/* esm.sh - zustand@5.0.15 */\n" +
		"import \"/react@>=18.0.0?target=es2022\";\n" +
		"export * from \"/zustand@5.0.15/es2022/zustand.bundle.mjs\";\n"
	f := &fetcher{bodies: map[string]string{
		"https://esm.sh/zustand?bundle&target=es2022": answer,
	}}
	url, err := assets.Resolve(context.Background(), f, "zustand")
	if err != nil {
		t.Fatalf("Resolve returned %v, want nil", err)
	}
	if url != "https://esm.sh/zustand@5.0.15/es2022/zustand.bundle.mjs" {
		t.Fatalf("Resolve returned %q", url)
	}
}

func TestResolveTakesTheAnswerWhenItHoldsTheModule(t *testing.T) {
	f := &fetcher{bodies: map[string]string{
		"https://esm.sh/tiny?bundle&target=es2022": "export const tiny = 1;\n",
	}}
	url, err := assets.Resolve(context.Background(), f, "tiny")
	if err != nil {
		t.Fatalf("Resolve returned %v, want nil", err)
	}
	if url != "https://esm.sh/tiny?bundle&target=es2022" {
		t.Fatalf("Resolve returned %q", url)
	}
}

func TestPinResolvesAPackageWithNoAddress(t *testing.T) {
	dir := t.TempDir()
	f := &fetcher{bodies: map[string]string{
		"https://esm.sh/zustand?bundle&target=es2022":             "export * from \"/zustand@5.0.15/es2022/zustand.bundle.mjs\";\n",
		"https://esm.sh/zustand@5.0.15/es2022/zustand.bundle.mjs": "export const create = () => 1;\n",
	}}
	if err := assets.PinJS(context.Background(), assets.PinConfig{Dir: dir, Fetch: f}, "zustand", ""); err != nil {
		t.Fatalf("PinJS returned %v, want nil", err)
	}
	if _, err := os.Stat(filepath.Join(dir, filepath.FromSlash(assets.VendorDir), "zustand.js")); err != nil {
		t.Fatalf("the vendor file is absent: %v", err)
	}
	lock, err := assets.LoadLock(dir)
	if err != nil {
		t.Fatalf("LoadLock returned %v, want nil", err)
	}
	pin, ok := lock.JS("zustand")
	if !ok || pin.URL != "https://esm.sh/zustand@5.0.15/es2022/zustand.bundle.mjs" {
		t.Fatalf("the lock holds %+v", pin)
	}
}

func TestPinNamesTheRepairForAPackageThatDoesNotResolve(t *testing.T) {
	f := &fetcher{bodies: map[string]string{}}
	err := assets.PinJS(context.Background(), assets.PinConfig{Dir: t.TempDir(), Fetch: f}, "absent-package", "")
	if err == nil {
		t.Fatal("PinJS returned nil, want a fault")
	}
	if !strings.Contains(err.Error(), "→") {
		t.Fatalf("the fault states no repair: %v", err)
	}
}
