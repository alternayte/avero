package assets

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"strings"
)

// The names of the Tailwind pipeline.
const (
	// BinDir holds the pinned binaries of one application.
	BinDir = ".avero/bin"
	// TailwindVersion is the version that a scaffolded application pins.
	TailwindVersion = "4.1.11"
	// TailwindRelease is the address of one standalone binary. The version,
	// the operating system and the architecture fill it.
	TailwindRelease = "https://github.com/tailwindlabs/tailwindcss/releases/download/v%s/tailwindcss-%s-%s"
)

// Tailwind drives the Tailwind standalone binary. The binary needs no Node.js,
// so tier 0 styles an application with Go, Docker and avero alone. See DX-9.
type Tailwind struct {
	// Version is the release to pin, such as 4.1.11.
	Version string
	// Dir is the root of the application.
	Dir string
	// Input is the stylesheet that Tailwind reads, relative to Dir.
	Input string
	// Asset is the name that the manifest records for the output. An empty
	// value is css/app.css.
	Asset string
	// Minify shrinks the stylesheet. A release build sets it.
	Minify bool
	// Fetch downloads the binary. A nil value downloads over HTTP.
	Fetch Fetcher
}

// version returns the pinned version.
func (t *Tailwind) version() string {
	if t.Version == "" {
		return TailwindVersion
	}
	return t.Version
}

// asset returns the name that the manifest records.
func (t *Tailwind) asset() string {
	if t.Asset == "" {
		return "css/app.css"
	}
	return t.Asset
}

// platform returns the operating system and the architecture of the release.
func platform() (string, string) {
	system := runtime.GOOS
	if system == "darwin" {
		system = "macos"
	}
	arch := runtime.GOARCH
	if arch == "amd64" {
		arch = "x64"
	}
	return system, arch
}

// URL returns the address of the binary for this machine.
func (t *Tailwind) URL() string {
	system, arch := platform()
	return fmt.Sprintf(TailwindRelease, t.version(), system, arch)
}

// Name returns the file name of the pinned binary.
func (t *Tailwind) Name() string {
	system, arch := platform()
	name := fmt.Sprintf("tailwindcss-%s-%s-%s", t.version(), system, arch)
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	return name
}

// Ensure returns the path of the binary. It downloads the binary one time and
// records the URL and the hash in the lock, so a second machine gets the same
// bytes. A binary that the lock already holds is verified, not downloaded.
func (t *Tailwind) Ensure(ctx context.Context) (string, error) {
	file := filepath.Join(t.Dir, filepath.FromSlash(BinDir), t.Name())
	lock, err := LoadLock(t.Dir)
	if err != nil {
		return "", err
	}
	pin, locked := lock.Bin("tailwindcss")
	if locked && pin.Version == t.version() && fileMatches(file, pin.SHA256) {
		return file, nil
	}

	fetch := t.Fetch
	if fetch == nil {
		fetch = HTTPFetcher(nil)
	}
	body, err := fetch.Fetch(ctx, t.URL())
	if err != nil {
		return "", fault(path.Join(BinDir, t.Name()),
			fmt.Sprintf("the Tailwind binary does not download from %s: %v", t.URL(), err),
			"Prove the network, or set the Tailwind version to a release that exists")
	}
	sum := sha256.Sum256(body)
	hash := hex.EncodeToString(sum[:])
	if locked && pin.Version == t.version() && pin.SHA256 != hash {
		return "", fault(LockName,
			fmt.Sprintf("the Tailwind binary does not match its hash: the release holds %s and %s records %s",
				hash[:12], LockName, pin.SHA256[:12]),
			fmt.Sprintf("Prove the change. Delete the binary entry from %s. Run the command again.", LockName))
	}
	if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
		return "", fault(BinDir, "the binary directory does not open",
			"Give the process the right to write the application directory")
	}
	if err := os.WriteFile(file, body, 0o755); err != nil {
		return "", fault(path.Join(BinDir, t.Name()), "the Tailwind binary does not write",
			"Give the process the right to write the application directory")
	}
	lock.SetBin("tailwindcss", Pin{URL: t.URL(), SHA256: hash, Version: t.version()})
	if err := lock.Save(); err != nil {
		return "", err
	}
	return file, nil
}

// Compile runs the binary and returns the stylesheet. The build hashes the
// bytes and writes them into the output directory, so the CSS reaches the
// manifest like every other asset.
func (t *Tailwind) Compile(ctx context.Context) ([]byte, error) {
	bin, err := t.Ensure(ctx)
	if err != nil {
		return nil, err
	}
	out, err := os.CreateTemp("", "avero-tailwind-*.css")
	if err != nil {
		return nil, fault(t.Input, "the temporary stylesheet does not open",
			"Give the process the right to write the temporary directory")
	}
	name := out.Name()
	_ = out.Close()
	defer func() { _ = os.Remove(name) }()

	args := []string{"-i", t.Input, "-o", name}
	if t.Minify {
		args = append(args, "--minify")
	}
	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.Dir = t.Dir
	cmd.Env = os.Environ()
	if body, err := cmd.CombinedOutput(); err != nil {
		return nil, fault(t.Input,
			fmt.Sprintf("Tailwind failed: %v\n%s", err, strings.TrimSpace(string(body))),
			"Repair the stylesheet that the message names. Run `avero build` again.")
	}
	css, err := os.ReadFile(name)
	if err != nil {
		return nil, fault(t.Input, "Tailwind wrote no stylesheet",
			"Prove the Tailwind version. Run `avero build` again.")
	}
	return css, nil
}
