# Basecoat UI Command Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add `avero ui add basecoat`, which puts the Basecoat component library into a new or an existing ssr application and renders the toasts of Avero in the Basecoat markup.

**Architecture:** One pin fetches the npm tarball of Basecoat and writes its `dist/` tree under `assets/vendor/basecoat/`. The stylesheet of the application imports the tree, so the pinned Tailwind standalone binary compiles one preflight and one theme. The server renders the toaster, because the MutationObserver of Basecoat initializes a toast that Datastar patches in.

**Tech Stack:** Go 1.26, templ, Datastar, the Tailwind 4.1.11 standalone binary, esbuild as a Go library, `archive/tar` and `compress/gzip` of the standard library.

**Spec:** `docs/superpowers/specs/2026-09-09-basecoat-ui-command-design.md`

## Global Constraints

- Write every comment, document and commit message in ASD-STE100. Active voice. One word for one meaning. No contraction.
- Write the test before the code that it judges. Never relax a test to pass the gate.
- No new third-party Go dependency. `archive/tar` and `compress/gzip` are in the standard library.
- An error names a path, a line and a column of a file that the user wrote, and it carries one sentence that states the repair. See DX-6 and DX-7.
- The Basecoat version is `1.0.2`. The default style is `vega`.
- The eight styles are `vega`, `nova`, `maia`, `lyra`, `mira`, `luma`, `sera`, `rhea`.
- The tarball address is `https://registry.npmjs.org/basecoat-css/-/basecoat-css-<version>.tgz`.
- Never edit a generated file by hand. Change the generator, then run `go run ./internal/cmd/averoexamples`.
- Run `just verify` before the last commit of the plan.

---

### Task 1: Prove that esbuild takes the bundle of Basecoat

The scripts of Basecoat are IIFE files and not ES modules. Step 4 of the command adds `import "./vendor/basecoat/js/all.min.js";` to `assets/js/app.js`. If esbuild refuses an IIFE import, the command needs another step, so this task runs first and changes nothing else.

**Files:**
- Test: `assets/build_test.go` (append)

**Interfaces:**
- Consumes: `assets.Build(ctx, Config)` and the helpers `write`, `project` and `config` of `assets/build_test.go`.
- Produces: nothing. This task proves a fact.

- [ ] **Step 1: Write the failing test**

Append to `assets/build_test.go`:

```go
func TestTheBuildTakesAnIIFEModule(t *testing.T) {
	dir := project(t)
	// The scripts of Basecoat are IIFE files. An application imports one for
	// its side effect, so the bundle must hold its body.
	write(t, dir, "assets/vendor/basecoat/js/all.min.js",
		"(() => { window.basecoat = { initAll: () => 1 }; })();\n")
	write(t, dir, "assets/js/app.js",
		"import \"../vendor/basecoat/js/all.min.js\";\ndocument.title = \"hi\";\n")

	if _, err := assets.Build(context.Background(), config(dir)); err != nil {
		t.Fatalf("Build returned %v, want nil", err)
	}
	body, err := os.ReadFile(filepath.Join(dir, "assets", "dist", "js", "app.js"))
	if err != nil {
		t.Fatalf("the bundle is absent: %v", err)
	}
	if !strings.Contains(string(body), "initAll") {
		t.Fatalf("the bundle holds no body of the module: %s", body)
	}
}
```

- [ ] **Step 2: Run the test**

Run: `go test ./assets -run TestTheBuildTakesAnIIFEModule -v`

Expected: PASS. esbuild bundles a relative import of an IIFE and keeps its side effect.

If it FAILS, stop the plan and report the message. The repair is a change to step 4 of section 6 of the spec, and the plan needs a new task before Task 5.

- [ ] **Step 3: Commit**

```bash
git add assets/build_test.go
git commit -m "Prove that the bundle takes an IIFE module

The scripts of Basecoat are IIFE files and not ES modules. The command adds a
relative import of one, so the pipeline must keep its side effect."
```

---

### Task 2: Record a package in the lock

`avero.lock` records a module under `js` and a binary under `bin`. A library arrives as a tree, so the lock needs a third map that records one URL, one hash and the list of the files that the pin wrote.

**Files:**
- Modify: `assets/lock.go:28-50` (the `Pin` struct, the `Lock` struct and `lockJSON`), `assets/lock.go:58-120` (`LoadLock`, the readers and the writers, `Save`)
- Test: `assets/pin_test.go` (append)

**Interfaces:**
- Consumes: `assets.LoadLock(dir)`, `assets.Pin`.
- Produces:
  - `Pin.Files []string` with the JSON name `files`, sorted, each one a slash path relative to the vendor directory of the package.
  - `func (l *Lock) Package(name string) (Pin, bool)`
  - `func (l *Lock) SetPackage(name string, pin Pin)`
  - The lock document gains the member `pkg`.

- [ ] **Step 1: Write the failing test**

Append to `assets/pin_test.go`:

```go
func TestTheLockRecordsAPackage(t *testing.T) {
	dir := t.TempDir()
	lock, err := assets.LoadLock(dir)
	if err != nil {
		t.Fatalf("LoadLock returned %v, want nil", err)
	}
	lock.SetPackage("basecoat", assets.Pin{
		URL:    "https://registry.example.com/basecoat-1.0.2.tgz",
		SHA256: "abc",
		Files:  []string{"basecoat.css", "js/all.min.js"},
	})
	if err := lock.Save(); err != nil {
		t.Fatalf("Save returned %v, want nil", err)
	}

	again, err := assets.LoadLock(dir)
	if err != nil {
		t.Fatalf("LoadLock returned %v, want nil", err)
	}
	pin, ok := again.Package("basecoat")
	if !ok {
		t.Fatal("the lock holds no package")
	}
	if pin.URL != "https://registry.example.com/basecoat-1.0.2.tgz" || pin.SHA256 != "abc" {
		t.Fatalf("the pin holds %+v", pin)
	}
	if len(pin.Files) != 2 || pin.Files[0] != "basecoat.css" {
		t.Fatalf("the pin records the files %v", pin.Files)
	}
}

func TestAnOlderLockWithNoPackageMemberReads(t *testing.T) {
	dir := t.TempDir()
	body := "{\n  \"js\": {\n    \"datastar\": {\n      \"url\": \"https://example.com/d.js\",\n      \"sha256\": \"aa\"\n    }\n  },\n  \"bin\": {}\n}\n"
	if err := os.WriteFile(filepath.Join(dir, "avero.lock"), []byte(body), 0o644); err != nil {
		t.Fatalf("WriteFile returned %v", err)
	}

	lock, err := assets.LoadLock(dir)
	if err != nil {
		t.Fatalf("LoadLock returned %v, want nil", err)
	}
	if _, ok := lock.JS("datastar"); !ok {
		t.Fatal("the lock lost the module of an older file")
	}
	if _, ok := lock.Package("basecoat"); ok {
		t.Fatal("the lock states a package that the file does not hold")
	}
}
```

- [ ] **Step 2: Run the tests to prove that they fail**

Run: `go test ./assets -run 'TestTheLockRecordsAPackage|TestAnOlderLock' -v`

Expected: FAIL with `lock.SetPackage undefined` and `pin.Files undefined`.

- [ ] **Step 3: Add the field, the map and the two methods**

In `assets/lock.go`, add the field to `Pin` after `Version`:

```go
	// Files names each file that a package pin wrote, as a slash path under
	// the vendor directory of the package. The list sorts, so two runs give
	// one lock. A module pin and a binary pin leave it empty.
	Files []string `json:"files,omitempty"`
```

Change `Lock` and `lockJSON`:

```go
type Lock struct {
	path string
	js   map[string]Pin
	bin  map[string]Pin
	pkg  map[string]Pin
}

// lockJSON is the wire shape of the lock.
type lockJSON struct {
	JS  map[string]Pin `json:"js"`
	Bin map[string]Pin `json:"bin"`
	Pkg map[string]Pin `json:"pkg,omitempty"`
}
```

In `LoadLock`, build the third map and read it:

```go
	l := &Lock{path: name, js: map[string]Pin{}, bin: map[string]Pin{}, pkg: map[string]Pin{}}
```

and after the loop that reads `doc.Bin`:

```go
	for name, pin := range doc.Pkg {
		l.pkg[name] = pin
	}
```

Add the two methods beside `Bin` and `SetBin`:

```go
// Package returns the pin of a library that arrived as a package.
func (l *Lock) Package(name string) (Pin, bool) {
	pin, ok := l.pkg[name]
	return pin, ok
}

// SetPackage records the pin of a library that arrived as a package.
func (l *Lock) SetPackage(name string, pin Pin) { l.pkg[name] = pin }
```

Change `Save` to write the third map:

```go
	body, err := json.MarshalIndent(lockJSON{JS: l.js, Bin: l.bin, Pkg: l.pkg}, "", "  ")
```

- [ ] **Step 4: Run the tests to prove that they pass**

Run: `go test ./assets -race -count=1`

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add assets/lock.go assets/pin_test.go
git commit -m "Record a package of many files in the lock

A module is one file and a library is a tree. The lock gains a third map that
records one address, one hash and the list of the files that the pin wrote, so
a review reads one entry and not the whole tree."
```

---

### Task 3: PinPackage

**Files:**
- Create: `assets/package.go`
- Test: `assets/package_test.go`

**Interfaces:**
- Consumes: `assets.PinConfig`, `assets.Fetcher`, `assets.LoadLock`, `assets.Pin`, `assets.fault(file, message, repair)`.
- Produces:
  - `const BasecoatVersion = "1.0.2"`
  - `const PackageDir = "assets/vendor"`
  - `func PinPackage(ctx context.Context, cfg PinConfig, name, url string) error`

  `PinPackage` fetches the gzip tarball at `url`, and writes each member whose path starts with `package/dist/` into `<Dir>/assets/vendor/<name>/`, with `package/dist/` removed from the path. It records the URL, the SHA-256 of the tarball and the sorted file list under `name` in the lock.

- [ ] **Step 1: Write the failing tests**

Create `assets/package_test.go`:

```go
package assets_test

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"os"
	"path/filepath"
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
		"package/dist/../../escape.css": "body{}\n",
	})}}

	err := assets.PinPackage(context.Background(), assets.PinConfig{Dir: dir, Fetch: f}, "basecoat", packageURL)
	if err == nil {
		t.Fatal("PinPackage returned nil for a path that leaves the directory")
	}
	if _, statErr := os.Stat(filepath.Join(dir, "escape.css")); statErr == nil {
		t.Fatal("the pin wrote a file outside the vendor directory")
	}
}
```

Add `"strings"` to the import block of the test file.

- [ ] **Step 2: Run the tests to prove that they fail**

Run: `go test ./assets -run TestPinPackage -v`

Expected: FAIL with `undefined: assets.PinPackage`.

- [ ] **Step 3: Write PinPackage**

Create `assets/package.go`:

```go
package assets

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
)

// BasecoatVersion is the release of Basecoat that `avero ui add basecoat`
// fetches. See the SDD, S11.
const BasecoatVersion = "1.0.2"

// PackageDir holds the tree of each library that `avero ui add` fetched,
// relative to the application root.
const PackageDir = "assets/vendor"

// distPrefix is the directory of an npm tarball that holds the released files.
// npm writes every member under a directory that it names package.
const distPrefix = "package/dist/"

// PinPackage fetches the released package of a library, writes its dist
// directory into the vendor directory, and records the address, the hash and
// the file list in the lock.
//
// One tarball carries the stylesheets and the scripts of a library, so one pin
// covers both and the build needs no network. See the SDD, S11.
//
// The command is idempotent: a name that the lock holds, whose files all exist
// and whose tarball gives the recorded hash, fetches nothing.
//
// The command fails when the bytes of a locked address give another hash,
// because that means the content of the address changed.
func PinPackage(ctx context.Context, cfg PinConfig, name, url string) error {
	lock, err := LoadLock(cfg.Dir)
	if err != nil {
		return err
	}
	pin, locked := lock.Package(name)
	if url == "" && locked {
		url = pin.URL
	}
	if url == "" {
		return fault(LockName, fmt.Sprintf("the package %q states no address", name),
			"Name the address of the package, or add the package one time with `avero ui add`")
	}

	root := filepath.Join(cfg.Dir, filepath.FromSlash(PackageDir), name)
	if locked && pin.URL == url && filesExist(root, pin.Files) {
		return nil
	}

	body, err := cfg.fetcher().Fetch(ctx, url)
	if err != nil {
		return fault(LockName, fmt.Sprintf("the package %q does not download from %s: %v", name, url, err),
			"Prove the address in a browser, then run the command again")
	}
	sum := sha256.Sum256(body)
	hash := hex.EncodeToString(sum[:])
	if locked && pin.URL == url && pin.SHA256 != hash {
		return fault(LockName,
			fmt.Sprintf("the package %q does not match its hash: %s holds %s and %s records %s",
				name, url, hash[:12], LockName, pin.SHA256[:12]),
			fmt.Sprintf("Prove the change, then delete the entry of %q from %s and run the command again", name, LockName))
	}

	files, err := extract(body, root, name)
	if err != nil {
		return err
	}
	if len(files) == 0 {
		return fault(LockName, fmt.Sprintf("the package %q holds no %s directory", name, distPrefix),
			"Prove that the address names the released package of the library")
	}
	sort.Strings(files)
	lock.SetPackage(name, Pin{URL: url, SHA256: hash, Files: files})
	return lock.Save()
}

// filesExist reports whether every file of a pin is present.
func filesExist(root string, files []string) bool {
	if len(files) == 0 {
		return false
	}
	for _, name := range files {
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(name))); err != nil {
			return false
		}
	}
	return true
}

// extract writes the dist directory of a gzip tarball into root. It returns
// the slash path of each file that it wrote, relative to root.
func extract(body []byte, root, name string) ([]string, error) {
	zip, err := gzip.NewReader(bytes.NewReader(body))
	if err != nil {
		return nil, fault(LockName, fmt.Sprintf("the package %q does not open as a gzip file", name),
			"Prove that the address names a tarball of npm, then run the command again")
	}
	defer func() { _ = zip.Close() }()

	var files []string
	in := tar.NewReader(zip)
	for {
		header, err := in.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fault(LockName, fmt.Sprintf("the package %q does not read as a tarball", name),
				"Prove that the address names a tarball of npm, then run the command again")
		}
		if header.Typeflag != tar.TypeReg {
			continue
		}
		member, ok := distMember(header.Name)
		if !ok {
			continue
		}
		if err := writeMember(root, member, in); err != nil {
			return nil, err
		}
		files = append(files, member)
	}
	return files, nil
}

// distMember returns the path of a member under the dist directory, and
// whether the member belongs there.
//
// It refuses a path that leaves the directory, so a tarball cannot write a
// file of its own choice. See the SDD, S11.
func distMember(raw string) (string, bool) {
	name := path.Clean(strings.TrimPrefix(raw, "./"))
	if !strings.HasPrefix(name, distPrefix) {
		return "", false
	}
	member := strings.TrimPrefix(name, distPrefix)
	if member == "" || strings.HasPrefix(member, "../") || strings.HasPrefix(member, "/") {
		return "", false
	}
	return member, true
}

// writeMember writes one file of the tarball under root.
func writeMember(root, member string, in io.Reader) error {
	file := filepath.Join(root, filepath.FromSlash(member))
	if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
		return fault(path.Join(PackageDir, member), "the vendor directory does not open",
			"Give the process the right to write the assets directory")
	}
	out, err := os.Create(file)
	if err != nil {
		return fault(path.Join(PackageDir, member), "the file does not write",
			"Give the process the right to write the assets directory")
	}
	defer func() { _ = out.Close() }()
	if _, err := io.Copy(out, io.LimitReader(in, MaxDownload)); err != nil {
		return fault(path.Join(PackageDir, member), "the file does not write",
			"Give the process the right to write the assets directory")
	}
	return nil
}
```

**Note on the traversal test.** `path.Clean` turns `package/dist/../../escape.css` into `escape.css`, which does not carry the prefix, so `distMember` returns false and the member is skipped. The tarball then writes no file, and `PinPackage` fails with "holds no package/dist/ directory". Both assertions of the test hold.

- [ ] **Step 4: Run the tests to prove that they pass**

Run: `go test ./assets -race -count=1`

Expected: PASS.

- [ ] **Step 5: Prove that the tree compiles with Tailwind**

This step needs the network one time. Run it by hand and record the result in the commit message.

```bash
cd $(mktemp -d) && mkdir -p assets/css
curl -sL -o bc.tgz https://registry.npmjs.org/basecoat-css/-/basecoat-css-1.0.2.tgz
mkdir -p assets/vendor/basecoat && tar xzf bc.tgz -C assets/vendor/basecoat --strip-components=2 package/dist
printf '@import "tailwindcss";\n@import "../vendor/basecoat/basecoat-vega.css";\n' > assets/css/app.css
curl -sL -o tw https://github.com/tailwindlabs/tailwindcss/releases/download/v4.1.11/tailwindcss-macos-arm64 && chmod +x tw
./tw -i assets/css/app.css -o out.css --minify && ls -l out.css
```

Expected: the compile ends with `Done`, and `out.css` holds about 288000 bytes.

- [ ] **Step 6: Commit**

```bash
git add assets/package.go assets/package_test.go
git commit -m "Fetch the released package of a library into the vendor directory

PinPackage reads the tarball of npm, writes its dist directory under
assets/vendor/<name>, and records one address, one hash and the file list. A
member whose path leaves the directory is refused, so a tarball writes no file
of its own choice.

The tree compiles with the pinned Tailwind standalone binary and needs no
Node.js."
```

---

### Task 4: The toaster component

The command writes `internal/ui/toaster.templ` into the application. The file holds the Basecoat markup of the toaster, so the flash cookie of Avero renders a Basecoat toast.

The file lives in this repository as a template, and the suffix keeps `templ generate` of this repository away from it.

**Files:**
- Create: `internal/cli/uifiles/toaster.templ.tmpl`
- Create: `internal/cli/uifiles/uifiles.go`
- Test: `internal/cli/uifiles/uifiles_test.go`

**Interfaces:**
- Consumes: nothing.
- Produces:
  - `package uifiles`
  - `var Basecoat embed.FS` which holds `toaster.templ.tmpl`
  - `func Toaster() []byte` which returns the body of the toaster component

- [ ] **Step 1: Write the failing test**

Create `internal/cli/uifiles/uifiles_test.go`:

```go
package uifiles_test

import (
	"strings"
	"testing"

	"github.com/alternayte/avero/internal/cli/uifiles"
)

func TestTheToasterHoldsTheBasecoatMarkup(t *testing.T) {
	body := string(uifiles.Toaster())
	for _, want := range []string{
		"package ui",
		"id=\"toaster\"",
		"class=\"toaster\"",
		"data-align=\"end\"",
		"data-category",
		"data-toast-cancel",
		"aria-atomic",
		"templ Toasts()",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("the component holds no %q", want)
		}
	}
}
```

- [ ] **Step 2: Run the test to prove that it fails**

Run: `go test ./internal/cli/uifiles -v`

Expected: FAIL with `no required module provides package`.

- [ ] **Step 3: Write the package and the component**

Create `internal/cli/uifiles/uifiles.go`:

```go
// Package uifiles holds the components that `avero ui add` writes into an
// application. The suffix of each file keeps the templ generator of this
// repository away from a template that it must not compile.
package uifiles

import (
	_ "embed"
)

//go:embed toaster.templ.tmpl
var toaster []byte

// Toaster returns the toaster component of Basecoat. The command writes it to
// internal/ui/toaster.templ of the application.
func Toaster() []byte { return toaster }
```

Create `internal/cli/uifiles/toaster.templ.tmpl`:

```
package ui

import "github.com/alternayte/avero"

// Toasts renders the toaster of Basecoat.
//
// The server renders each toast, because Basecoat watches the document and
// initializes a toast that arrives later. A toast therefore gets its timer,
// its pause on hover and its dismiss with no script of the application. The
// stylesheet places the toaster, so the position needs no script at all.
//
// The container renders even when it holds no toast, so a patch of Datastar
// replaces #toaster in outer mode and needs no second target.
templ Toasts() {
	<div id="toaster" class="toaster" data-align="end">
		for _, toast := range avero.Toasts(ctx) {
			@toastItem(toast)
		}
	</div>
}

// toastItem renders one message in the markup that Basecoat states.
//
// The footer carries the dismiss button, because the click handler of the
// toaster closes a toast for a button of a footer only. A pointer that rests
// on a toast pauses its timer, so a toast with no button never hides.
templ toastItem(toast avero.Toast) {
	<div class="toast" role={ toastRole(toast.Level) } aria-atomic="true" data-category={ toast.Level }>
		<div class="toast-content">
			@toastIcon(toast.Level)
			<section>
				<h2>{ toast.Message }</h2>
			</section>
			<footer>
				<button type="button" class="btn h-6 text-xs px-2.5 rounded-sm" data-variant="outline" data-toast-cancel>Dismiss</button>
			</footer>
		</div>
	</div>
}

// toastRole returns the ARIA role of one level. A fault interrupts a person,
// and every other message waits.
func toastRole(level string) string {
	if level == avero.ToastError {
		return "alert"
	}
	return "status"
}

// toastIcon renders the icon of one level. The icons are the four that
// Basecoat draws for its own categories.
templ toastIcon(level string) {
	switch level {
		case avero.ToastSuccess:
			<svg aria-hidden="true" xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="10"></circle><path d="m9 12 2 2 4-4"></path></svg>
		case avero.ToastError:
			<svg aria-hidden="true" xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="10"></circle><path d="m15 9-6 6"></path><path d="m9 9 6 6"></path></svg>
		case avero.ToastWarning:
			<svg aria-hidden="true" xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="m21.73 18-8-14a2 2 0 0 0-3.48 0l-8 14A2 2 0 0 0 4 21h16a2 2 0 0 0 1.73-3"></path><path d="M12 9v4"></path><path d="M12 17h.01"></path></svg>
		default:
			<svg aria-hidden="true" xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="10"></circle><path d="M12 16v-4"></path><path d="M12 8h.01"></path></svg>
	}
}
```

- [ ] **Step 4: Run the test to prove that it passes**

Run: `go test ./internal/cli/uifiles -race -count=1`

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/cli/uifiles
git commit -m "Hold the toaster of Basecoat as a file that the command writes

The server renders each toast, because Basecoat watches the document and
initializes a toast that arrives later. The footer carries the dismiss button,
because the click handler of the toaster closes a toast for a button of a
footer only."
```

---

### Task 5: The command, without the patch of the layout

This task adds `avero ui add basecoat`. It pins the package, changes the two entry files, and writes the toaster. Task 6 adds the patch of the layout.

**Files:**
- Create: `internal/cli/ui.go`
- Modify: `internal/cli/cli.go:68-72` (the command list, after the `js` entry)
- Test: `internal/cli/ui_test.go`

**Interfaces:**
- Consumes: `assets.PinPackage`, `assets.BasecoatVersion`, `assets.PackageDir`, `uifiles.Toaster`, `cli.LoadProject`, `cli.Streams`, `cli.failf`, `cli.fail`, `cli.dirOf`.
- Produces:
  - `func runUI(ctx context.Context, s Streams, args []string) int`
  - `var BasecoatStyles = []string{"vega", "nova", "maia", "lyra", "mira", "luma", "sera", "rhea"}`
  - `func BasecoatURL(version string) string`

- [ ] **Step 1: Write the failing tests**

Create `internal/cli/ui_test.go`:

```go
package cli_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// uiProject writes the files that `avero ui add basecoat` reads.
func uiProject(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	writeFile(t, dir, "avero.json", "{\"name\":\"blog\",\"shape\":\"ssr\",\"hypermedia\":\"datastar\"}\n")
	writeFile(t, dir, "assets/css/app.css", "@import \"tailwindcss\";\n\nbody {\n    margin: 0;\n}\n")
	writeFile(t, dir, "assets/js/app.js", "import \"datastar\";\n")
	return dir
}

func TestUIAddBasecoatWritesTheStylesheetTheScriptAndTheComponent(t *testing.T) {
	dir := uiProject(t)

	code, out, errOut := run(t, dir, "ui", "add", "basecoat")
	if code != 0 {
		t.Fatalf("the command gave %d and said %q", code, errOut)
	}

	css := read(t, dir, "assets/css/app.css")
	if !strings.Contains(css, "@import \"../vendor/basecoat/basecoat-vega.css\";") {
		t.Fatalf("the stylesheet is %q", css)
	}
	if strings.Index(css, "tailwindcss") > strings.Index(css, "basecoat-vega") {
		t.Fatal("the Basecoat import stands before the Tailwind import")
	}
	js := read(t, dir, "assets/js/app.js")
	if !strings.Contains(js, "import \"../vendor/basecoat/js/all.min.js\";") {
		t.Fatalf("the script is %q", js)
	}
	if _, err := os.Stat(filepath.Join(dir, "internal", "ui", "toaster.templ")); err != nil {
		t.Fatalf("the component is absent: %v", err)
	}
	if !strings.Contains(out, "basecoat") {
		t.Fatalf("the command said %q", out)
	}
}

func TestUIAddBasecoatRunsTwoTimesWithOneResult(t *testing.T) {
	dir := uiProject(t)

	if code, _, errOut := run(t, dir, "ui", "add", "basecoat"); code != 0 {
		t.Fatalf("the first run gave %d and said %q", code, errOut)
	}
	first := read(t, dir, "assets/css/app.css")
	if code, _, errOut := run(t, dir, "ui", "add", "basecoat"); code != 0 {
		t.Fatalf("the second run gave %d and said %q", code, errOut)
	}
	if second := read(t, dir, "assets/css/app.css"); second != first {
		t.Fatalf("the second run changed the stylesheet to %q", second)
	}
	if strings.Count(read(t, dir, "assets/js/app.js"), "all.min.js") != 1 {
		t.Fatal("the second run added the import again")
	}
}

func TestUIAddBasecoatTakesAStyle(t *testing.T) {
	dir := uiProject(t)

	if code, _, errOut := run(t, dir, "ui", "add", "basecoat", "--style", "nova"); code != 0 {
		t.Fatalf("the command gave %d and said %q", code, errOut)
	}
	if css := read(t, dir, "assets/css/app.css"); !strings.Contains(css, "basecoat-nova.css") {
		t.Fatalf("the stylesheet is %q", css)
	}
}

func TestUIAddBasecoatRefusesAnUnknownStyle(t *testing.T) {
	dir := uiProject(t)

	code, _, errOut := run(t, dir, "ui", "add", "basecoat", "--style", "orion")
	if code != 1 {
		t.Fatalf("the command gave %d, want 1", code)
	}
	if !strings.Contains(errOut, "vega") {
		t.Fatalf("the fault names no known style: %q", errOut)
	}
}

func TestUIAddRefusesAnUnknownLibrary(t *testing.T) {
	dir := uiProject(t)

	code, _, errOut := run(t, dir, "ui", "add", "bootstrap")
	if code != 1 {
		t.Fatalf("the command gave %d, want 1", code)
	}
	if !strings.Contains(errOut, "basecoat") {
		t.Fatalf("the fault names no known library: %q", errOut)
	}
}

func TestUIAddRefusesAShapeThatServesJSON(t *testing.T) {
	dir := uiProject(t)
	writeFile(t, dir, "avero.json", "{\"name\":\"orders\",\"shape\":\"api\"}\n")

	code, _, errOut := run(t, dir, "ui", "add", "basecoat")
	if code != 1 {
		t.Fatalf("the command gave %d, want 1", code)
	}
	if !strings.Contains(errOut, "ssr") {
		t.Fatalf("the fault states no repair: %q", errOut)
	}
}
```

**The helpers.** `internal/cli/cli_test.go:15` already holds `run`, and both files are in the package `cli_test`, so do not write `run` again. It holds no `writeFile` and no `read`. Add these two to `internal/cli/ui_test.go`:

```go
// writeFile puts one file into the tree of a test.
func writeFile(t *testing.T, dir, name, body string) {
	t.Helper()
	full := filepath.Join(dir, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatalf("MkdirAll returned %v", err)
	}
	if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
		t.Fatalf("WriteFile returned %v", err)
	}
}

// read returns the body of one file of the tree of a test.
func read(t *testing.T, dir, name string) string {
	t.Helper()
	body, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(name)))
	if err != nil {
		t.Fatalf("ReadFile returned %v", err)
	}
	return string(body)
}
```

**The network.** These tests must not reach the network. `assets.PinConfig` carries a `Fetch` field, so `runUI` must take the fetcher from a package variable that a test replaces:

```go
// uiFetch reads the package of a library. A test replaces it, so the gate
// needs no network.
var uiFetch assets.Fetcher
```

The test sets it in `TestMain` or in each test with a helper that restores it. Add to `internal/cli/ui_test.go`:

```go
func TestMain(m *testing.M) { os.Exit(m.Run()) }
```

only if `internal/cli` holds no `TestMain` already. `internal/cli/cli_test.go` holds none today, so check before you add it. Set the fetcher in `uiProject`:

```go
	cli.SetUIFetcher(fakeBasecoat(t))
	t.Cleanup(func() { cli.SetUIFetcher(nil) })
```

with

```go
// fakeBasecoat answers the address of the package with a small tarball of the
// shape that npm publishes.
func fakeBasecoat(t *testing.T) assets.Fetcher { ... }
```

Build the tarball with `archive/tar` and `compress/gzip`, exactly as `assets/package_test.go` does, and hold `package/dist/basecoat-vega.css`, `package/dist/basecoat-nova.css` and `package/dist/js/all.min.js`.

- [ ] **Step 2: Run the tests to prove that they fail**

Run: `go test ./internal/cli -run TestUIAdd -v`

Expected: FAIL with `undefined: cli.SetUIFetcher` and an unknown command.

- [ ] **Step 3: Write the command**

Create `internal/cli/ui.go`:

```go
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
```

In `internal/cli/cli.go`, add the command after the `js` entry:

```go
		{Name: "ui", Usage: "avero ui add <library> [--style <name>]",
			Summary: "add a component library to this application", Run: runUI},
```

- [ ] **Step 4: Run the tests to prove that they pass**

Run: `go test ./internal/cli -race -count=1`

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/cli/ui.go internal/cli/ui_test.go internal/cli/cli.go
git commit -m "Add the command that puts a component library into an application

avero ui add basecoat pins the package, imports the stylesheet after the
Tailwind import, imports the script, and writes the toaster. Every step is
idempotent, so a second run changes nothing."
```

---

### Task 6: Patch the layout

The command must move `@Toasts()` to the end of `<body>`, and it must remove the `templ Toasts()` block of the scaffold, because Task 4 writes a second definition of that name.

**Correction to the spec.** Section 6.1 states one change. The layout of the scaffold defines `templ Toasts()` itself, so the patch needs two changes: move the call, and delete the definition. A file that holds both definitions does not compile.

**Files:**
- Modify: `internal/cli/ui.go`
- Test: `internal/cli/ui_test.go` (append)
- Read: `scaffold/templates/ssr/internal/ui/layout.templ.tmpl`

**Interfaces:**
- Consumes: the command of Task 5.
- Produces: `func patchLayout(dir string, s Streams) error`, which returns nil in both cases. It prints what it changed, or the lines to paste.

- [ ] **Step 1: Write the failing tests**

Append to `internal/cli/ui_test.go`. `scaffoldLayout` must hold the exact body that `scaffold/templates/ssr/internal/ui/layout.templ.tmpl` writes for an application named `blog` with Datastar. Read that file and copy it.

```go
func TestUIAddBasecoatPatchesALayoutThatItRecognizes(t *testing.T) {
	dir := uiProject(t)
	writeFile(t, dir, "internal/ui/layout.templ", scaffoldLayout)

	if code, out, errOut := run(t, dir, "ui", "add", "basecoat"); code != 0 {
		t.Fatalf("the command gave %d, said %q and %q", code, out, errOut)
	}

	layout := read(t, dir, "internal/ui/layout.templ")
	if strings.Contains(layout, "templ Toasts()") {
		t.Fatal("the layout still defines Toasts, so the application holds two definitions")
	}
	body := strings.Index(layout, "@Toasts()")
	main := strings.Index(layout, "</main>")
	if body < 0 || body < main {
		t.Fatalf("the call of Toasts stands inside main:\n%s", layout)
	}
	if !strings.Contains(layout, "</body>") {
		t.Fatalf("the patch broke the layout:\n%s", layout)
	}
}

func TestUIAddBasecoatLeavesAChangedLayoutAlone(t *testing.T) {
	dir := uiProject(t)
	changed := "package ui\n\ntempl Page(title string, body templ.Component) {\n\t<html><body>@body</body></html>\n}\n"
	writeFile(t, dir, "internal/ui/layout.templ", changed)

	code, out, errOut := run(t, dir, "ui", "add", "basecoat")
	if code != 0 {
		t.Fatalf("the command gave %d and said %q", code, errOut)
	}
	if read(t, dir, "internal/ui/layout.templ") != changed {
		t.Fatal("the command changed a layout that it does not recognize")
	}
	if !strings.Contains(out, "@Toasts()") {
		t.Fatalf("the command printed no line to paste: %q", out)
	}
}
```

- [ ] **Step 2: Run the tests to prove that they fail**

Run: `go test ./internal/cli -run TestUIAddBasecoatPatches -v` and `go test ./internal/cli -run TestUIAddBasecoatLeaves -v`

Expected: FAIL. The first one fails because the layout still defines `Toasts`.

- [ ] **Step 3: Write the patch**

Add to `internal/cli/ui.go`, and call `patchLayout(dir, s)` in `runUI` after `writeToaster`:

```go
// scaffoldToasts is the block that the ssr layout of the scaffold defines. The
// command removes it, because the toaster component defines the same name.
const scaffoldToasts = "// Toasts renders the messages of this response."

// patchLayout moves the call of Toasts to the end of the body and removes the
// definition that the scaffold wrote.
//
// The command patches a layout that still matches the scaffold. A person who
// changed the layout owns it, so the command changes nothing and prints the
// two lines. See the design, D2.
func patchLayout(dir string, s Streams) error {
	file := filepath.Join(dir, "internal", "ui", "layout.templ")
	body, err := os.ReadFile(file)
	if err != nil {
		return nil
	}
	text := string(body)
	if !strings.Contains(text, "@Toasts()") || !strings.Contains(text, scaffoldToasts) {
		printLayoutLines(s)
		return nil
	}

	// Remove the call from its place inside main.
	out := []string{}
	for _, line := range strings.Split(text, "\n") {
		if strings.TrimSpace(line) == "@Toasts()" {
			continue
		}
		out = append(out, line)
	}
	text = strings.Join(out, "\n")

	// Put the call before the end of the body, because the toaster is a fixed
	// element of the page and not of the content.
	at := strings.Index(text, "</body>")
	if at < 0 {
		printLayoutLines(s)
		return nil
	}
	text = text[:at] + "\t\t\t@Toasts()\n\t\t" + text[at:]

	// Remove the definition of the scaffold, which reaches to the end of the
	// file.
	if cut := strings.Index(text, scaffoldToasts); cut >= 0 {
		text = strings.TrimRight(text[:cut], "\n") + "\n"
	}

	if err := os.WriteFile(file, []byte(text), 0o644); err != nil {
		return fmt.Errorf("avero ui: internal/ui/layout.templ does not write: %w\n  → Give the process the right to write the application directory", err)
	}
	_, _ = fmt.Fprintln(s.Out, "patched internal/ui/layout.templ")
	return nil
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
```

**Note.** The definition of `Toasts` is the last block of the scaffold layout, so the cut reaches the end of the file. Read `scaffold/templates/ssr/internal/ui/layout.templ.tmpl` and prove it. If a later block follows it, cut to the end of the block instead.

- [ ] **Step 4: Run the tests to prove that they pass**

Run: `go test ./internal/cli -race -count=1`

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/cli/ui.go internal/cli/ui_test.go
git commit -m "Patch a layout that the command recognizes

The command moves the call of Toasts to the end of the body and removes the
definition of the scaffold, because the toaster component defines that name. A
layout that a person changed does not change, and the command prints the two
lines."
```

---

### Task 7: The blog example

The example proves the command on every run of the gate, so the command and the example never drift.

**Files:**
- Modify: `internal/cmd/averoexamples/main.go` (the `write` function, after `cli.Drel`)
- Modify: `scaffold/templates/ssr/internal/ui/posts.templ.tmpl`
- Modify: `scaffold/templates/ssr/assets/css/app.css`
- Modify: `examples/blog/acceptance_test.go` through `scaffold/templates/ssr/acceptance_test.go.tmpl`
- Regenerate: `examples/blog`, `examples/board`, `examples/orders`

**Interfaces:**
- Consumes: `cli.Run` with the arguments `ui add basecoat`.
- Produces: an example that carries `assets/vendor/basecoat/` and `internal/ui/toaster.templ`.

- [ ] **Step 1: Write the failing acceptance test**

`scaffold/templates/ssr/acceptance_test.go.tmpl:104` already holds
`TestAValidFormWritesAPostAndShowsTheToast`, and it reads the text of the toast
only. Strengthen it, so it reads the markup of Basecoat. Its helpers are
`app(t)`, `get(t, h, path, cookies)`, `post(t, h, path, form, cookies)` and
`token(t, rec)`. Write no new helper.

Replace the last assertion of that test:

```go
	if !strings.Contains(body, "The post is saved") {
		t.Fatalf("the list holds no toast:\n%s", body)
	}
	// The toaster of Basecoat holds the message. The server renders it,
	// because Basecoat watches the document and initializes a toast that
	// arrives later. See the design of 2026-09-09.
	if !strings.Contains(body, `id="toaster"`) {
		t.Fatalf("the list holds no toaster:\n%s", body)
	}
	if !strings.Contains(body, `data-category="success"`) {
		t.Fatalf("the toast states no category:\n%s", body)
	}
	if !strings.Contains(body, "data-toast-cancel") {
		t.Fatalf("the toast carries no dismiss button:\n%s", body)
	}
```

Add a second test after it, which proves that the container stands on a page
that shows no message. A patch of Datastar replaces `#toaster` in outer mode,
so the container must always exist:

```go
func TestTheToasterStandsWithNoToast(t *testing.T) {
	rec := get(t, app(t), "/", nil)

	body := rec.Body.String()
	if !strings.Contains(body, `id="toaster"`) {
		t.Fatalf("the page holds no toaster:\n%s", body)
	}
	if strings.Contains(body, `class="toast"`) {
		t.Fatalf("the page holds a toast although no message exists:\n%s", body)
	}
}
```

The success toast comes from `c.Success("The post is saved")` at
`scaffold/templates/ssr/internal/features/posts/handlers.go.tmpl:36`. It needs
no change.

- [ ] **Step 2: Run it to prove that it fails**

```bash
go run ./internal/cmd/averoexamples
cd examples/blog && go test ./... -run 'TestAValidFormWritesAPostAndShowsTheToast|TestTheToasterStandsWithNoToast' -v
```

Expected: FAIL. The page holds the old `<ul class="toasts">` markup.

- [ ] **Step 3: Run the command inside the example generator**

In `internal/cmd/averoexamples/main.go`, in `write`, after the call of `cli.Drel` that follows the migration, add:

```go
	// The blog proves `avero ui add basecoat` on every run of the gate, so the
	// command and the example never drift. The pin is idempotent, so a tree
	// that the repository already holds needs no network.
	if e.Shape == "ssr" {
		if code := cli.Run(ctx, cli.Streams{Out: os.Stdout, Err: os.Stderr, Dir: dir}, []string{"ui", "add", "basecoat"}); code != 0 {
			return fmt.Errorf("avero ui add basecoat failed in %s", dir)
		}
		// The component is new, so templ writes its Go file.
		if err := cli.Drel(ctx, dir); err != nil {
			return err
		}
	}
```

- [ ] **Step 4: Give the example the Basecoat classes**

In `scaffold/templates/ssr/assets/css/app.css`, remove the rules that Basecoat replaces, and keep the file at the two imports and the layout of the page:

```css
@import "tailwindcss";

body {
    margin: 0;
}

header,
main {
    max-width: 40rem;
    margin: 0 auto;
    padding: 1rem;
}
```

The command adds the Basecoat import after the Tailwind import, so this file states no Basecoat line.

In `scaffold/templates/ssr/internal/ui/posts.templ.tmpl`, put the Basecoat classes on the form, the fields and the buttons:

- `<button type="submit">Save</button>` becomes `<button type="submit" class="btn">Save</button>`
- each Delete button gains `class="btn" data-variant="destructive"`
- `<input .../>` gains `class="input"`
- `<textarea ...>` gains `class="textarea"`
- `<span class="error">` becomes `<span class="text-destructive text-sm">`
- the `<p>` of a field becomes `<div class="grid gap-2">`, and the `<label>` gains `class="label"`

- [ ] **Step 5: Write the examples again and run the gate of the example**

```bash
go run ./internal/cmd/averoexamples
cd examples/blog && go test ./... -race -count=1
```

Expected: PASS, including the two tests of the toaster.

- [ ] **Step 6: Prove that the check is deterministic**

```bash
go run ./internal/cmd/averoexamples -check
```

Expected: exit 0. A second write gives the same tree.

- [ ] **Step 7: Commit**

```bash
git add scaffold internal/cmd/averoexamples examples
git commit -m "Give the blog the components of Basecoat

The generator of the examples runs `avero ui add basecoat` for the ssr shape,
so the gate proves the command on every run. The acceptance test posts the
form, follows the redirect, and reads one toast of the success category inside
the toaster."
```

---

### Task 8: The documents

**Files:**
- Create: `docs/ui.md`
- Modify: `docs/assets.md`, `docs/views.md`, `README.md`, `CHANGELOG.md`
- Modify: `docs/internal/sdd.md` (S11 and S14)

- [ ] **Step 1: Write `docs/ui.md`**

State the command, the eight styles, the rule of the patch, the vendor tree, and the toaster. State R1 of the design: the toaster belongs to the application after the command wrote it, so a person owns the repair when a later version of Basecoat changes its markup.

- [ ] **Step 2: Add a section to `docs/assets.md`**

Add "The package of a library" after the section that states `avero js pin`. State that one tarball carries the stylesheets and the scripts, that the lock records one hash and the file list, and that the build needs no network.

- [ ] **Step 3: Add the toaster to `docs/views.md`**

State that the server renders each toast, that Basecoat watches the document and initializes a toast that arrives later, and that every toast carries a dismiss button because a pointer that rests on a toast pauses its timer.

- [ ] **Step 4: Name the command in `README.md`**

Add one line to the command list:

```
avero ui add <lib>    add a component library, such as basecoat
```

- [ ] **Step 5: Record the change in `CHANGELOG.md`**

Add an entry under `## Unreleased`, in the shape that the file already uses.

- [ ] **Step 6: Add the two lines to the SDD**

`docs/internal/sdd.md` is local only. Add one line to S11 for `PinPackage` and one line to S14 for `avero ui add`.

- [ ] **Step 7: Run the whole gate**

```bash
just verify
```

Expected: every step passes. Record the DX budgets in `artifacts/verification.md` if the gate writes them.

- [ ] **Step 8: Commit**

```bash
git add docs README.md CHANGELOG.md artifacts
git commit -m "State the command that adds a component library

The documents name the command, the eight styles, the rule of the patch and
the toaster."
```
