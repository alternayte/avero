package dev

import (
	"context"
	"fmt"
	"io/fs"
	"os/exec"
	"path/filepath"
	"strings"
)

// TemplCommand is the tool that watches the templ files. Avero wraps it and
// writes no generator and no watcher of its own. See the SDD, S15.
const TemplCommand = "templ"

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
	path, err := exec.LookPath(TemplCommand)
	if err != nil {
		return nil, fmt.Errorf("avero dev: the tree holds a templ file and the templ binary is absent\n  → Run `go install github.com/a-h/templ/cmd/templ@latest`, then run `avero dev` again")
	}
	cmd := exec.CommandContext(ctx, path, "generate", "--watch")
	cmd.Dir = dir
	cmd.Stdout = out
	cmd.Stderr = out
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("avero dev: templ does not start: %w\n  → Run `templ generate --watch` by hand and repair the fault that it names", err)
	}
	return stopper{stop: func() error {
		if cmd.Process == nil {
			return nil
		}
		_ = cmd.Process.Kill()
		_, err := cmd.Process.Wait()
		return err
	}}, nil
}

// stopper is one process that the loop stops.
type stopper struct {
	stop func() error
}

// Stop ends the process.
func (s stopper) Stop() error { return s.stop() }
