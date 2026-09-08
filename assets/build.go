package assets

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"

	"github.com/evanw/esbuild/pkg/api"
)

// Tier states how the pipeline resolves an import.
type Tier int

const (
	// TierBundled is the default. It needs no Node.js. It resolves a
	// relative path and a vendored package only. See DX-9.
	TierBundled Tier = iota
	// TierNode adds a package.json, so esbuild resolves a bare specifier
	// against node_modules. `avero assets init` writes the file.
	TierNode
	// TierExternal runs a command of the application. The command must
	// write the output directory and a manifest with the same shape.
	TierExternal
)

// The directories of the pipeline, relative to the application root.
const (
	// SourceDir holds the source of the assets.
	SourceDir = "assets"
	// OutDir holds the built assets and the manifest.
	OutDir = "assets/dist"
	// VendorDir holds the modules that `avero js pin` fetched.
	VendorDir = "assets/js/vendor"
)

// Config states one build.
type Config struct {
	// Dir is the root of the application. An empty value is the working
	// directory.
	Dir string
	// Entries names each entry point, relative to the source directory,
	// such as js/app.js and css/app.css.
	Entries []string
	// Source is the source directory. An empty value is assets.
	Source string
	// Out is the output directory. An empty value is assets/dist.
	Out string
	// Base is the URL prefix of an asset. An empty value is /assets/.
	Base string
	// Tier states how the pipeline resolves an import.
	Tier Tier
	// Command runs the build of TierExternal. The first element is the
	// program and the rest are its arguments. It runs in Dir.
	Command []string
	// Minify shrinks the output. A release build sets it.
	Minify bool
	// Tailwind compiles the stylesheet with the standalone binary. A nil
	// value leaves the stylesheet to esbuild.
	Tailwind *Tailwind
}

// source returns the source directory.
func (c Config) source() string {
	if c.Source == "" {
		return SourceDir
	}
	return c.Source
}

// out returns the output directory.
func (c Config) out() string {
	if c.Out == "" {
		return OutDir
	}
	return c.Out
}

// Build bundles the entry points and writes the output directory and the
// manifest. It returns the manifest that the application reads.
//
// Every tier writes one shape, so a page reads the same manifest whichever
// tier built it.
func Build(ctx context.Context, cfg Config) (*Manifest, error) {
	dir, err := filepath.Abs(cfg.Dir)
	if err != nil {
		return nil, fault(cfg.Dir, "the application directory does not resolve",
			"Pass the directory that holds the assets directory")
	}
	cfg.Dir = dir
	if cfg.Tier == TierExternal {
		return buildExternal(ctx, cfg)
	}
	return buildBundled(ctx, cfg)
}

// buildBundled runs esbuild for each entry point. It builds one entry at a
// time, so each output maps to one name with no rule to read.
func buildBundled(ctx context.Context, cfg Config) (*Manifest, error) {
	outDir := filepath.Join(cfg.Dir, filepath.FromSlash(cfg.out()))
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return nil, fault(cfg.out(), "the output directory does not open",
			"Give the process the right to write the output directory")
	}
	m := NewManifest(cfg.Base)
	var faults []*Fault
	if cfg.Tailwind != nil {
		css, err := cfg.Tailwind.Compile(ctx)
		if err != nil {
			return nil, err
		}
		if err := record(m, outDir, cfg.Tailwind.asset(), css); err != nil {
			return nil, err
		}
	}
	for _, entry := range cfg.Entries {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		files, entryFaults := bundle(cfg, outDir, entry)
		faults = append(faults, entryFaults...)
		for _, f := range files {
			if err := record(m, outDir, f.name, f.body); err != nil {
				return nil, err
			}
		}
	}
	if len(faults) > 0 {
		return nil, &Faults{Faults: faults}
	}
	if err := writeManifest(outDir, m); err != nil {
		return nil, err
	}
	return m, nil
}

// record writes one built file and puts it in the manifest.
//
// The manifest holds the path of the asset and its base name, because the SDD
// writes ui.Asset("app.css") and a page can also ask for css/app.css. The
// first entry of a base name wins, so two files with one base name stay apart
// under their paths.
func record(m *Manifest, outDir, name string, body []byte) error {
	written, err := writeAsset(outDir, name, body)
	if err != nil {
		return err
	}
	m.Set(written)
	if alias := path.Base(name); alias != name && !m.Has(alias) {
		written.Source = alias
		m.Set(written)
	}
	return nil
}

// output is one built file before the hash.
type output struct {
	// name is the name of the asset, relative to the source directory.
	name string
	// body is the built content.
	body []byte
}

// bundle builds one entry point.
func bundle(cfg Config, outDir, entry string) ([]output, []*Fault) {
	abs := filepath.Join(cfg.Dir, filepath.FromSlash(cfg.source()), filepath.FromSlash(entry))
	if _, err := os.Stat(abs); err != nil {
		return nil, []*Fault{{
			File:    path.Join(cfg.source(), entry),
			Message: fmt.Sprintf("the entry point %s is absent", entry),
			Repair:  "Write the file, or delete the entry from the assets configuration",
		}}
	}
	opts := api.BuildOptions{
		AbsWorkingDir: cfg.Dir,
		EntryPoints:   []string{abs},
		Outdir:        outDir,
		Bundle:        true,
		Write:         false,
		Format:        api.FormatESModule,
		Target:        api.ES2022,
		Platform:      api.PlatformBrowser,
		Loader: map[string]api.Loader{
			".png": api.LoaderFile, ".jpg": api.LoaderFile, ".jpeg": api.LoaderFile,
			".gif": api.LoaderFile, ".svg": api.LoaderFile, ".webp": api.LoaderFile,
			".woff": api.LoaderFile, ".woff2": api.LoaderFile, ".ttf": api.LoaderFile,
		},
		LogLevel: api.LogLevelSilent,
	}
	if cfg.Minify {
		opts.MinifyWhitespace = true
		opts.MinifyIdentifiers = true
		opts.MinifySyntax = true
	}
	if cfg.Tier == TierBundled {
		opts.Plugins = []api.Plugin{vendorPlugin(cfg)}
	}
	result := api.Build(opts)
	if len(result.Errors) > 0 {
		return nil, messageFaults(cfg.Dir, result.Errors)
	}
	return outputs(entry, result.OutputFiles), nil
}

// outputs maps the files of one build to their asset names.
//
// A JS entry can emit a CSS file beside it, and a font or an image that the
// source imports becomes a file of its own. The name of the entry decides the
// first case, and the name that esbuild chose decides the second.
func outputs(entry string, files []api.OutputFile) []output {
	dir := path.Dir(entry)
	stem := strings.TrimSuffix(path.Base(entry), path.Ext(entry))
	out := make([]output, 0, len(files))
	for _, f := range files {
		base := path.Base(filepath.ToSlash(f.Path))
		name := path.Join(dir, base)
		if strings.HasPrefix(base, stem+".") {
			name = path.Join(dir, stem+path.Ext(base))
		}
		out = append(out, output{name: name, body: f.Contents})
	}
	return out
}

// vendorPlugin resolves a bare specifier against the vendor directory. Tier 0
// resolves nothing else, so a build needs no node_modules. See DX-9.
func vendorPlugin(cfg Config) api.Plugin {
	vendor := filepath.Join(cfg.Dir, filepath.FromSlash(VendorDir))
	return api.Plugin{
		Name: "avero-vendor",
		Setup: func(build api.PluginBuild) {
			build.OnResolve(api.OnResolveOptions{Filter: `^[^./]`},
				func(args api.OnResolveArgs) (api.OnResolveResult, error) {
					for _, candidate := range []string{
						args.Path + ".js",
						filepath.Join(args.Path, "index.js"),
						args.Path,
					} {
						full := filepath.Join(vendor, filepath.FromSlash(candidate))
						if info, err := os.Stat(full); err == nil && !info.IsDir() {
							return api.OnResolveResult{Path: full}, nil
						}
					}
					return api.OnResolveResult{Errors: []api.Message{{
						Text: fmt.Sprintf("the package %q is not vendored. "+
							"Run `avero js pin %s` to fetch it into %s, "+
							"or run `avero assets init` to resolve it against node_modules",
							args.Path, args.Path, VendorDir),
					}}}, nil
				})
		},
	}
}

// messageFaults turns esbuild messages into faults. A position names the file
// of the person, never a file inside Avero. See DX-6.
func messageFaults(dir string, msgs []api.Message) []*Fault {
	out := make([]*Fault, 0, len(msgs))
	for _, msg := range msgs {
		f := &Fault{Message: msg.Text, Repair: repairOf(msg)}
		if msg.Location != nil {
			f.File = relative(dir, msg.Location.File)
			f.Line = msg.Location.Line
			f.Column = msg.Location.Column + 1
		}
		out = append(out, f)
	}
	return out
}

// repairOf returns the repair sentence of one message. The vendor plugin
// writes its own repair into the text, so the fault states it one time.
func repairOf(msg api.Message) string {
	if strings.Contains(msg.Text, "avero js pin") {
		return "Run the command that the message names"
	}
	return "Repair the file that the position names, then run `avero build` again"
}

// relative returns the path of a file inside the application.
func relative(dir, file string) string {
	if rel, err := filepath.Rel(dir, file); err == nil && !strings.HasPrefix(rel, "..") {
		return rel
	}
	return file
}

// writeAsset writes one built file under its hashed name.
func writeAsset(outDir, name string, body []byte) (Entry, error) {
	hash := Hash(body)
	file := HashedName(name, hash)
	full := filepath.Join(outDir, filepath.FromSlash(file))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		return Entry{}, fault(file, "the output directory does not open",
			"Give the process the right to write the output directory")
	}
	if err := os.WriteFile(full, body, 0o644); err != nil {
		return Entry{}, fault(file, "the asset does not write",
			"Give the process the right to write the output directory")
	}
	return Entry{
		Source:    name,
		File:      file,
		Hash:      hash,
		Size:      int64(len(body)),
		Integrity: Integrity(body),
	}, nil
}

// writeManifest writes the manifest into the output directory.
func writeManifest(outDir string, m *Manifest) error {
	body, err := json.Marshal(m)
	if err != nil {
		return fault(ManifestName, "the manifest does not encode",
			"Report the fault, because a manifest always encodes")
	}
	if err := os.WriteFile(filepath.Join(outDir, ManifestName), append(body, '\n'), 0o644); err != nil {
		return fault(ManifestName, "the manifest does not write",
			"Give the process the right to write the output directory")
	}
	return nil
}

// buildExternal runs the command of tier 2 and reads the manifest that it
// wrote. The command owns the whole build, so Avero adds nothing to it.
func buildExternal(ctx context.Context, cfg Config) (*Manifest, error) {
	if len(cfg.Command) == 0 {
		return nil, fault("", "the external bundler names no command",
			"Set the bundler command in the assets configuration, or use the default bundler")
	}
	cmd := exec.CommandContext(ctx, cfg.Command[0], cfg.Command[1:]...)
	cmd.Dir = cfg.Dir
	cmd.Env = os.Environ()
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fault("", fmt.Sprintf("the bundler command failed: %v\n%s", err, strings.TrimSpace(string(out))),
			"Run the bundler command by hand and repair the fault that it prints")
	}
	name := filepath.Join(cfg.Dir, filepath.FromSlash(cfg.out()), ManifestName)
	body, err := os.ReadFile(name)
	if err != nil {
		return nil, fault(path.Join(cfg.out(), ManifestName),
			fmt.Sprintf("the bundler command wrote no %s", ManifestName),
			"Make the command write the manifest with the shape of assets/schema.json")
	}
	var m Manifest
	if err := json.Unmarshal(body, &m); err != nil {
		return nil, fault(path.Join(cfg.out(), ManifestName),
			"the manifest of the bundler command does not parse",
			"Make the command write the manifest with the shape of assets/schema.json")
	}
	return &m, nil
}
