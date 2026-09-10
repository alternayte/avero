package dev

import (
	"context"
	"fmt"
	"io/fs"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// TemplCommand is the tool that watches the templ files. Avero wraps it and
// writes no generator and no watcher of its own. See the SDD, S15.
//
// The application carries the tool in its go.mod, so `go tool templ` runs the
// version that the application pins and a person installs nothing.
var TemplCommand = []string{"go", "tool", "templ", "generate", "--watch"}

// HasTempl reports a tree that holds a templ file.
func HasTempl(dir string) bool {
	found := false
	_ = filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if path != dir && skipDir(d.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.EqualFold(filepath.Ext(path), ".templ") {
			found = true
			return filepath.SkipAll
		}
		return nil
	})
	return found
}

// StartTempl runs `templ generate --watch` in the tree.
//
// The tool writes the Go file of each templ file. The loop then sees the Go
// file change and rebuilds, so the two steps compose and Avero writes no
// generator of its own.
//
// A tree with no templ file starts nothing. A tree that holds one and finds no
// templ binary returns a fault that states the repair.
func StartTempl(ctx context.Context, dir string, out interface{ Write([]byte) (int, error) }) (Process, error) {
	if !HasTempl(dir) {
		return nil, nil
	}
	cmd := exec.CommandContext(ctx, TemplCommand[0], TemplCommand[1:]...)
	cmd.Dir = dir
	cmd.Stdout = out
	cmd.Stderr = out
	// `go tool templ` starts the tool as a child of its own. The loop kills
	// the group, so no process stays behind and holds the output.
	Group(cmd)
	cmd.WaitDelay = 2 * time.Second
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("avero dev: templ does not start: %w%s", err,
			hint("Run `go get -tool github.com/a-h/templ/cmd/templ`. Run `avero dev` again."))
	}
	return stopper{stop: func() error {
		KillGroup(cmd)
		_ = cmd.Wait()
		return nil
	}}, nil
}

// stopper is one process that the loop stops.
type stopper struct {
	stop func() error
}

// Stop ends the process.
func (s stopper) Stop() error { return s.stop() }
