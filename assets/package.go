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

// maxPackageFiles bounds the count of members that a pin writes. Basecoat
// 1.0.2 holds 133 members, of which 106 stand under dist, so this bound gives
// room for a larger library and still stops a tarball that never ends.
const maxPackageFiles = 10000

// maxPackageBytes bounds the total of the bytes that a pin writes for one
// package. The uncompressed dist of Basecoat 1.0.2 is under 3 MB, so this
// bound gives room for a larger library and still stops a tarball that
// expands to an unbounded size.
const maxPackageBytes = 64 << 20

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
			"Prove the address in a browser. Run the command again.")
	}
	sum := sha256.Sum256(body)
	hash := hex.EncodeToString(sum[:])
	if locked && pin.URL == url && pin.SHA256 != hash {
		return fault(LockName,
			fmt.Sprintf("the package %q does not match its hash: %s holds %s and %s records %s",
				name, url, hash[:12], LockName, pin.SHA256[:12]),
			fmt.Sprintf("Prove the change. Delete the entry of %q from %s. Run the command again.", name, LockName))
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
	var total int64
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
		if len(files)+1 > maxPackageFiles {
			return nil, fault(LockName,
				fmt.Sprintf("the package %q holds more than %d files under %s", name, maxPackageFiles, distPrefix),
				"Prove that the address names the released package of the library")
		}
		total += header.Size
		if total > maxPackageBytes {
			return nil, fault(LockName,
				fmt.Sprintf("the package %q holds more than %d bytes of files under %s", name, maxPackageBytes, distPrefix),
				"Prove that the address names the released package of the library")
		}
		if err := writeMember(root, member, in, header.Size); err != nil {
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

// writeMember writes one file of the tarball under root. It copies exactly
// size bytes, so a tarball that ends before it declared its full size fails
// with a fault instead of leaving a short file that the pin calls a success.
func writeMember(root, member string, in io.Reader, size int64) error {
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
	if _, err := io.CopyN(out, in, size); err != nil {
		return fault(path.Join(PackageDir, member), "the file ends before its declared size",
			"Prove that the address names an entire tarball of npm")
	}
	return nil
}
