package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/alternayte/avero/cli/pipeline"
)

// ProjectFile is the file that states the shape of an application and the
// entry points of its assets. `avero new` writes it.
const ProjectFile = "avero.json"

// Project is the content of avero.json.
type Project struct {
	// Name is the name of the application.
	Name string `json:"name"`
	// Shape is ssr, spa or api.
	Shape string `json:"shape"`
	// Hypermedia is datastar or htmx. An api holds none.
	Hypermedia string `json:"hypermedia,omitempty"`
	// Assets states the asset pipeline.
	Assets ProjectAssets `json:"assets"`
	// Migrations is the directory that holds the migration files.
	Migrations string `json:"migrations"`
}

// ProjectAssets states the asset pipeline of one application.
type ProjectAssets struct {
	// Entries names each entry point, relative to the assets directory.
	Entries []string `json:"entries"`
	// Tier is bundled, node or external.
	Tier string `json:"tier,omitempty"`
	// Command runs the external bundler of tier 2. An empty value runs the
	// build script of the package manager.
	Command []string `json:"command,omitempty"`
	// PackageManager names the tool that reads the dependencies and runs the
	// scripts of the front end: npm, bun, pnpm or yarn. An empty value reads
	// the packageManager member of package.json, then the lock file, then
	// npm.
	PackageManager string `json:"packageManager,omitempty"`
	// Install reads the dependencies of the front end. An empty value runs
	// the install command of the package manager.
	Install []string `json:"install,omitempty"`
	// Dev runs the development server of the front end. An empty value runs
	// its dev script.
	Dev []string `json:"dev,omitempty"`
	// API writes the client of the front end from openapi.json. An empty
	// value runs its api script.
	API []string `json:"api,omitempty"`
	// Dir is the directory of the external bundler, relative to the
	// application. An empty value runs it in the application directory.
	Dir string `json:"dir,omitempty"`
	// Manifest states whether the bundler writes the manifest of Avero. A
	// bundler that writes its own index document, such as Vite, writes none,
	// and the application serves the build with assets.SPA.
	Manifest *bool `json:"manifest,omitempty"`
	// Tailwind states the stylesheet and the pinned version. An empty input
	// runs no Tailwind.
	Tailwind ProjectTailwind `json:"tailwind"`
}

// ProjectTailwind states the Tailwind build.
type ProjectTailwind struct {
	// Input is the stylesheet that Tailwind reads.
	Input string `json:"input,omitempty"`
	// Version is the pinned release of the standalone binary.
	Version string `json:"version,omitempty"`
	// Asset is the name that the manifest records.
	Asset string `json:"asset,omitempty"`
}

// LoadProject reads avero.json from dir.
//
// An absent file gives the defaults, so a command works in a directory that a
// person wrote by hand. A file that exists states the whole asset pipeline: a
// member that it does not name is empty, so a project that names no Tailwind
// input runs no Tailwind.
func LoadProject(dir string) (*Project, error) {
	p := &Project{}
	body, err := os.ReadFile(filepath.Join(dir, ProjectFile))
	if errors.Is(err, os.ErrNotExist) {
		p.Name = filepath.Base(mustAbs(dir))
		p.Shape = "ssr"
		p.Migrations = "migrations"
		p.Assets = ProjectAssets{
			Entries:  []string{"js/app.js"},
			Tailwind: ProjectTailwind{Input: "assets/css/app.css", Version: pipeline.TailwindVersion, Asset: "css/app.css"},
		}
		return p, nil
	}
	if err != nil {
		return nil, fmt.Errorf("avero: %s does not open\n  → Give the process the right to read the application directory", ProjectFile)
	}
	if err := json.Unmarshal(body, p); err != nil {
		return nil, fmt.Errorf("avero: %s does not parse: %w\n  → Repair the file, or delete it and let the defaults apply", ProjectFile, err)
	}
	if p.Name == "" {
		p.Name = filepath.Base(mustAbs(dir))
	}
	if p.Shape == "" {
		p.Shape = "ssr"
	}
	if p.Migrations == "" {
		p.Migrations = "migrations"
	}
	if p.Assets.Tailwind.Input != "" {
		if p.Assets.Tailwind.Version == "" {
			p.Assets.Tailwind.Version = pipeline.TailwindVersion
		}
		if p.Assets.Tailwind.Asset == "" {
			p.Assets.Tailwind.Asset = "css/app.css"
		}
	}
	return p, nil
}

// Save writes avero.json into dir.
func (p *Project) Save(dir string) error {
	body, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return fmt.Errorf("avero: %s does not encode", ProjectFile)
	}
	return os.WriteFile(filepath.Join(dir, ProjectFile), append(body, '\n'), 0o644)
}

// assetConfig returns the asset configuration of the project.
func (p *Project) assetConfig(dir string, minify bool) pipeline.Config {
	cfg := pipeline.Config{
		Dir:        dir,
		Entries:    p.Assets.Entries,
		Minify:     minify,
		Command:    p.Assets.Command,
		CommandDir: p.Assets.Dir,
		NoManifest: p.Assets.Manifest != nil && !*p.Assets.Manifest,
	}
	switch p.Assets.Tier {
	case "node":
		cfg.Tier = pipeline.TierNode
	case "external":
		cfg.Tier = pipeline.TierExternal
	default:
		cfg.Tier = pipeline.TierBundled
	}
	if p.Assets.Tailwind.Input != "" {
		cfg.Tailwind = &pipeline.Tailwind{
			Version: p.Assets.Tailwind.Version,
			Dir:     dir,
			Input:   p.Assets.Tailwind.Input,
			Asset:   p.Assets.Tailwind.Asset,
			Minify:  minify,
		}
	}
	return cfg
}

// mustAbs returns the absolute form of a path, or the path itself.
func mustAbs(dir string) string {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return dir
	}
	return abs
}

// dirOf returns the working directory of one run.
func dirOf(s Streams) string {
	if s.Dir == "" {
		return "."
	}
	return s.Dir
}
