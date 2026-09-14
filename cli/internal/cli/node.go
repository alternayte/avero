package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// The package managers that a front end can use. Each one reads the
// dependencies and runs the scripts of package.json.
const (
	// NPM is the package manager of Node.js.
	NPM = "npm"
	// Bun is the package manager and the runtime of Bun.
	Bun = "bun"
	// PNPM is the package manager pnpm.
	PNPM = "pnpm"
	// Yarn is the package manager Yarn.
	Yarn = "yarn"
)

// PackageManagers names every package manager that Avero drives.
var PackageManagers = []string{NPM, Bun, PNPM, Yarn}

// NodeTool runs the package manager of one front end.
//
// Avero states no package manager of its own. The application names the tool
// in avero.json, or the front end states it in the packageManager member of
// its package.json, or its lock file names it. An application of Bun therefore
// never calls npm.
type NodeTool struct {
	// Name is npm, bun, pnpm or yarn.
	Name string
	// Dir is the directory of the front end.
	Dir string
	// install is the command that reads the dependencies.
	install []string
	// scripts holds the command of one script, by its name in avero.json.
	scripts map[string][]string
}

// The scripts of package.json that Avero runs.
const (
	// ScriptBuild builds the front end for a release.
	ScriptBuild = "build"
	// ScriptDev runs the development server of the front end.
	ScriptDev = "dev"
	// ScriptAPI writes the client of the front end from openapi.json.
	ScriptAPI = "api"
)

// NodeToolOf returns the package manager of the front end of one project.
//
// front is the directory of the front end. The order of the sources is the
// order of the intent: avero.json states the tool of this application, the
// packageManager member of package.json states the tool of the front end, and
// the lock file states the tool that wrote it. npm is the last answer, because
// every machine that holds Node.js holds npm.
func NodeToolOf(project *Project, front string) (NodeTool, error) {
	name := strings.TrimSpace(project.Assets.PackageManager)
	if name == "" {
		name = packageManagerOf(front)
	}
	if name == "" {
		name = lockFileManager(front)
	}
	if name == "" {
		name = NPM
	}
	tool := NodeTool{Name: name, Dir: front, scripts: map[string][]string{}}
	switch name {
	case NPM:
		// The flags leave out the report of the registry, which no build
		// reads.
		tool.install = []string{NPM, "install", "--no-audit", "--no-fund"}
	case Bun, PNPM, Yarn:
		tool.install = []string{name, "install"}
	default:
		return NodeTool{}, fmt.Errorf(
			"avero: the package manager %q is not known\n  → Write one of %s in the packageManager member of %s, or name the command in the install, dev, api and command members",
			name, strings.Join(PackageManagers, ", "), ProjectFile)
	}
	for script, command := range map[string][]string{
		ScriptBuild: project.Assets.Command,
		ScriptDev:   project.Assets.Dev,
		ScriptAPI:   project.Assets.API,
	} {
		if len(command) > 0 {
			tool.scripts[script] = command
		}
	}
	if len(project.Assets.Install) > 0 {
		tool.install = project.Assets.Install
	}
	return tool, nil
}

// Install returns the command that reads the dependencies.
func (n NodeTool) Install() []string { return n.install }

// Script returns the command that runs one script of package.json. The
// application replaces it in avero.json.
func (n NodeTool) Script(name string) []string {
	if command, ok := n.scripts[name]; ok {
		return command
	}
	return []string{n.Name, "run", name}
}

// Command builds the command of one script in the directory of the front end.
// It reports the absent tool before it runs anything, so the message names the
// program and the repair and not an error of the operating system.
func (n NodeTool) Command(ctx context.Context, command []string) (*exec.Cmd, error) {
	if len(command) == 0 {
		return nil, fmt.Errorf("avero: the front end names no command\n  → Name the command in %s", ProjectFile)
	}
	if _, err := exec.LookPath(command[0]); err != nil {
		return nil, fmt.Errorf(
			"avero: the program %q is not on the path\n  → Install %s, or name another package manager in the packageManager member of %s",
			command[0], command[0], ProjectFile)
	}
	cmd := exec.CommandContext(ctx, command[0], command[1:]...)
	cmd.Dir = n.Dir
	cmd.Env = os.Environ()
	return cmd, nil
}

// HasScript reports a script that package.json states. A front end that states
// no script for the client of the API runs none.
func (n NodeTool) HasScript(name string) bool {
	if _, ok := n.scripts[name]; ok {
		return true
	}
	scripts := readPackageJSON(n.Dir).Scripts
	_, ok := scripts[name]
	return ok
}

// packageFile is the part of package.json that Avero reads.
type packageFile struct {
	// PackageManager is the member that Corepack states, such as
	// "bun@1.2.0".
	PackageManager string `json:"packageManager"`
	// Scripts holds the commands that the front end states.
	Scripts map[string]string `json:"scripts"`
}

// readPackageJSON reads package.json of the front end. An absent file gives
// the empty value, because a front end that holds none runs no script.
func readPackageJSON(front string) packageFile {
	var out packageFile
	body, err := os.ReadFile(filepath.Join(front, "package.json"))
	if err != nil {
		return out
	}
	_ = json.Unmarshal(body, &out)
	return out
}

// packageManagerOf returns the tool that package.json names in its
// packageManager member. Corepack states the form name@version.
func packageManagerOf(front string) string {
	name, _, _ := strings.Cut(readPackageJSON(front).PackageManager, "@")
	return strings.TrimSpace(name)
}

// lockFileManager returns the tool that wrote the lock file of the front end.
func lockFileManager(front string) string {
	for _, pair := range []struct{ file, name string }{
		{"bun.lock", Bun},
		{"bun.lockb", Bun},
		{"pnpm-lock.yaml", PNPM},
		{"yarn.lock", Yarn},
		{"package-lock.json", NPM},
	} {
		if _, err := os.Stat(filepath.Join(front, pair.file)); err == nil {
			return pair.name
		}
	}
	return ""
}
