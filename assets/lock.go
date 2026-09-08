package assets

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"sort"
	"time"
)

// LockName is the file that records every pinned URL and its hash.
const LockName = "avero.lock"

// MaxDownload is the largest body that the fetcher reads. The Tailwind
// standalone binary holds about 80 megabytes, so the limit gives it room and
// still stops a body that never ends.
const MaxDownload = 512 << 20

// Pin records one fetched file.
type Pin struct {
	// URL is the address that the file came from.
	URL string `json:"url"`
	// SHA256 is the hash of the bytes, in hexadecimal.
	SHA256 string `json:"sha256"`
	// Version is the version of a binary. A module leaves it empty.
	Version string `json:"version,omitempty"`
}

// Lock is avero.lock. It records the URL and the hash of every file that a
// command fetched, so a second machine gets the same bytes.
type Lock struct {
	path string
	js   map[string]Pin
	bin  map[string]Pin
}

// lockJSON is the wire shape of the lock.
type lockJSON struct {
	JS  map[string]Pin `json:"js"`
	Bin map[string]Pin `json:"bin"`
}

// LoadLock reads the lock of this application. An absent file gives an empty
// lock, so the first pin needs no file.
func LoadLock(dir string) (*Lock, error) {
	name := filepath.Join(dir, LockName)
	l := &Lock{path: name, js: map[string]Pin{}, bin: map[string]Pin{}}
	body, err := os.ReadFile(name)
	if errors.Is(err, os.ErrNotExist) {
		return l, nil
	}
	if err != nil {
		return nil, fault(LockName, "the lock does not open",
			"Give the process the right to read the lock")
	}
	var doc lockJSON
	if err := json.Unmarshal(body, &doc); err != nil {
		return nil, fault(LockName, "the lock does not parse",
			"Repair the lock by hand, or delete it and run `avero js pin` for each package")
	}
	for name, pin := range doc.JS {
		l.js[name] = pin
	}
	for name, pin := range doc.Bin {
		l.bin[name] = pin
	}
	return l, nil
}

// JS returns the pin of a module.
func (l *Lock) JS(name string) (Pin, bool) {
	pin, ok := l.js[name]
	return pin, ok
}

// Bin returns the pin of a binary.
func (l *Lock) Bin(name string) (Pin, bool) {
	pin, ok := l.bin[name]
	return pin, ok
}

// Names returns the name of each pinned module, in alphabetical order.
func (l *Lock) Names() []string {
	out := make([]string, 0, len(l.js))
	for name := range l.js {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

// SetJS records the pin of a module.
func (l *Lock) SetJS(name string, pin Pin) { l.js[name] = pin }

// SetBin records the pin of a binary.
func (l *Lock) SetBin(name string, pin Pin) { l.bin[name] = pin }

// Save writes the lock. The document sorts its keys, so two runs give the same
// file. See AN-4.
func (l *Lock) Save() error {
	body, err := json.MarshalIndent(lockJSON{JS: l.js, Bin: l.bin}, "", "  ")
	if err != nil {
		return fault(LockName, "the lock does not encode",
			"Report the fault, because a lock always encodes")
	}
	if err := os.WriteFile(l.path, append(body, '\n'), 0o644); err != nil {
		return fault(LockName, "the lock does not write",
			"Give the process the right to write the application directory")
	}
	return nil
}

// Fetcher reads a URL. The pipeline takes it as a field, so a test needs no
// network. See design rule 3.
type Fetcher interface {
	// Fetch returns the bytes of the URL.
	Fetch(ctx context.Context, url string) ([]byte, error)
}

// httpFetcher reads a URL over HTTP.
type httpFetcher struct{ client *http.Client }

// HTTPFetcher returns the fetcher that a command uses. A nil client gives a
// client with a timeout of one minute.
func HTTPFetcher(client *http.Client) Fetcher {
	if client == nil {
		client = &http.Client{Timeout: time.Minute}
	}
	return httpFetcher{client: client}
}

// Fetch reads the URL and returns the body.
func (f httpFetcher) Fetch(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	res, err := f.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("the server answered %s", res.Status)
	}
	return io.ReadAll(io.LimitReader(res.Body, MaxDownload))
}

// PinConfig states one pin.
type PinConfig struct {
	// Dir is the root of the application. An empty value is the working
	// directory.
	Dir string
	// Fetch reads the URL. A nil value reads over HTTP.
	Fetch Fetcher
}

// fetcher returns the fetcher of the configuration.
func (c PinConfig) fetcher() Fetcher {
	if c.Fetch == nil {
		return HTTPFetcher(nil)
	}
	return c.Fetch
}

// PinJS fetches a bundled ES module, writes it into the vendor directory, and
// records the URL and the hash in the lock.
//
// The command is idempotent: a pin that the lock already holds, with a vendor
// file that matches the hash, fetches nothing. An empty url reads the URL of
// the lock, so `avero js pin nanostores` repairs an absent file.
//
// The command fails when the bytes of a locked URL give another hash, because
// that means the content of the URL changed. See S11.
func PinJS(ctx context.Context, cfg PinConfig, name, url string) error {
	lock, err := LoadLock(cfg.Dir)
	if err != nil {
		return err
	}
	pin, locked := lock.JS(name)
	switch {
	case url == "" && !locked:
		return fault(LockName, fmt.Sprintf("the lock holds no pin of the module %q", name),
			fmt.Sprintf("Run `avero js pin %s <url>` with the address of the bundled module", name))
	case url == "":
		url = pin.URL
	}

	file := filepath.Join(cfg.Dir, filepath.FromSlash(VendorDir), name+".js")
	if locked && pin.URL == url && fileMatches(file, pin.SHA256) {
		return nil
	}

	body, err := cfg.fetcher().Fetch(ctx, url)
	if err != nil {
		return fault(LockName, fmt.Sprintf("the module %q does not download from %s: %v", name, url, err),
			"Prove the address in a browser, then run the command again")
	}
	sum := sha256.Sum256(body)
	hash := hex.EncodeToString(sum[:])
	if locked && pin.URL == url && pin.SHA256 != hash {
		return fault(LockName,
			fmt.Sprintf("the module %q does not match its hash: %s holds %s and %s records %s",
				name, url, hash[:12], LockName, pin.SHA256[:12]),
			fmt.Sprintf("Prove the change, then delete the entry of %q from %s and run the command again", name, LockName))
	}

	if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
		return fault(path.Join(VendorDir, name+".js"), "the vendor directory does not open",
			"Give the process the right to write the assets directory")
	}
	if err := os.WriteFile(file, body, 0o644); err != nil {
		return fault(path.Join(VendorDir, name+".js"), "the module does not write",
			"Give the process the right to write the assets directory")
	}
	lock.SetJS(name, Pin{URL: url, SHA256: hash})
	return lock.Save()
}

// fileMatches reports whether the file holds these bytes.
func fileMatches(file, hash string) bool {
	body, err := os.ReadFile(file)
	if err != nil {
		return false
	}
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:]) == hash
}
