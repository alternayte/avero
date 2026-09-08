package assets

import (
	_ "embed"
	"encoding/json"
	"io/fs"
	"path"
	"sort"
	"strings"
)

// The names of the manifest.
const (
	// ManifestName is the file that every tier writes into the output
	// directory.
	ManifestName = "manifest.json"
	// ManifestSchemaID names the schema that a manifest follows. See
	// assets/schema.json.
	ManifestSchemaID = "https://avero.dev/schema/asset-manifest/v1"
	// DefaultBase is the URL prefix that the handler serves.
	DefaultBase = "/assets/"
)

// ManifestSchema holds assets/schema.json. Every tier writes a document that
// this schema accepts, so a tier 2 command must write the same shape. See
// AN-3.
//
//go:embed schema.json
var ManifestSchema []byte

// Entry is one built asset.
type Entry struct {
	// Source is the name that the page asks for, such as app.css.
	Source string `json:"source"`
	// File is the built name with the hash, such as app.a1b2c3.css.
	File string `json:"file"`
	// Hash is the content hash of the file.
	Hash string `json:"hash"`
	// Size is the number of bytes of the file.
	Size int64 `json:"size"`
	// Integrity is the subresource integrity value of the file.
	Integrity string `json:"integrity"`
}

// Manifest maps the name of an asset to the built file. `avero build` writes
// it, and the application reads it one time at start.
type Manifest struct {
	// Base is the URL prefix of every asset, such as /assets/.
	Base string
	// entries holds one entry for each asset, keyed by source name.
	entries map[string]Entry
}

// NewManifest returns an empty manifest. An empty base becomes /assets/.
func NewManifest(base string) *Manifest {
	if base == "" {
		base = DefaultBase
	}
	if !strings.HasSuffix(base, "/") {
		base += "/"
	}
	return &Manifest{Base: base, entries: make(map[string]Entry, 8)}
}

// Set records one entry.
func (m *Manifest) Set(e Entry) {
	if m.entries == nil {
		m.entries = make(map[string]Entry, 8)
	}
	m.entries[e.Source] = e
}

// Entry returns the entry of this asset.
func (m *Manifest) Entry(name string) (Entry, bool) {
	e, ok := m.entries[name]
	return e, ok
}

// Names returns the source name of each asset, in alphabetical order. See
// AN-4.
func (m *Manifest) Names() []string {
	out := make([]string, 0, len(m.entries))
	for name := range m.entries {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

// Len returns the number of assets.
func (m *Manifest) Len() int { return len(m.entries) }

// Asset returns the URL path of an asset.
//
// A name that the manifest does not hold returns the unhashed path, so a page
// still renders while a person repairs the build. `avero doctor` reports the
// absent entry before the process starts. See DX-8.
//
//	<link rel="stylesheet" href={ ui.Asset("app.css") }>
func (m *Manifest) Asset(name string) string {
	if m == nil {
		return path.Join(DefaultBase, name)
	}
	if e, ok := m.entries[name]; ok {
		return m.Base + e.File
	}
	return m.Base + strings.TrimPrefix(name, "/")
}

// Has reports whether the manifest holds this asset.
func (m *Manifest) Has(name string) bool {
	_, ok := m.entries[name]
	return ok
}

// manifestJSON is the wire shape of a manifest.
type manifestJSON struct {
	Schema string           `json:"schema"`
	Base   string           `json:"base"`
	Assets map[string]Entry `json:"assets"`
}

// MarshalJSON emits the manifest with a stable schema. See AN-3.
func (m *Manifest) MarshalJSON() ([]byte, error) {
	entries := m.entries
	if entries == nil {
		entries = map[string]Entry{}
	}
	base := m.Base
	if base == "" {
		base = DefaultBase
	}
	return json.MarshalIndent(manifestJSON{
		Schema: ManifestSchemaID, Base: base, Assets: entries,
	}, "", "  ")
}

// UnmarshalJSON reads a manifest that any tier wrote.
func (m *Manifest) UnmarshalJSON(b []byte) error {
	var doc manifestJSON
	if err := json.Unmarshal(b, &doc); err != nil {
		return err
	}
	base := doc.Base
	if base == "" {
		base = DefaultBase
	}
	if !strings.HasSuffix(base, "/") {
		base += "/"
	}
	m.Base = base
	m.entries = make(map[string]Entry, len(doc.Assets))
	for name, e := range doc.Assets {
		if e.Source == "" {
			e.Source = name
		}
		m.entries[name] = e
	}
	return nil
}

// LoadManifest reads the manifest at this path from a file system. The
// application passes the embedded file system that holds assets/dist.
//
//	//go:embed all:assets/dist
//	var dist embed.FS
//	m, err := assets.LoadManifest(dist, "assets/dist/manifest.json")
func LoadManifest(fsys fs.FS, name string) (*Manifest, error) {
	b, err := fs.ReadFile(fsys, name)
	if err != nil {
		return nil, fault(name, "the asset manifest is absent",
			"Run `avero build` to write the manifest, and embed the output directory")
	}
	var m Manifest
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, fault(name, "the asset manifest does not parse",
			"Delete the output directory and run `avero build` again")
	}
	return &m, nil
}
