package dev

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"
)

// Debounce is the time that the watcher waits after a change, so one save of
// an editor gives one rebuild.
const Debounce = 20 * time.Millisecond

// watcher reports the source files that change.
//
// It uses fsnotify, which reads the notifications of the operating system.
// Avero writes no watcher of its own. See the SDD, S15.
type watcher struct {
	dir   string
	inner *fsnotify.Watcher
}

// newWatcher opens a watcher on the tree of the application. It adds each
// directory, because fsnotify reports one directory and not a whole tree.
func newWatcher(dir string) (*watcher, error) {
	inner, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("avero dev: the watcher does not open: %w\n  → Raise the file limit of the shell, then run the command again", err)
	}
	w := &watcher{dir: dir, inner: inner}
	if err := w.addTree(); err != nil {
		_ = inner.Close()
		return nil, err
	}
	return w, nil
}

// addTree adds every directory of the application to the watcher.
func (w *watcher) addTree() error {
	return filepath.WalkDir(w.dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || !d.IsDir() {
			return nil
		}
		if path != w.dir && skipDir(d.Name()) {
			return filepath.SkipDir
		}
		if err := w.inner.Add(path); err != nil {
			return fmt.Errorf("avero dev: the watcher does not read %s: %w\n  → Give the process the right to read the application directory", path, err)
		}
		return nil
	})
}

// Close ends the watcher.
func (w *watcher) Close() error { return w.inner.Close() }

// next returns the source files of one change. It waits for the debounce, so
// one save gives one rebuild, and it adds a new directory to the watcher.
func (w *watcher) next(ctx context.Context) ([]string, error) {
	files := map[string]bool{}
	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case err := <-w.inner.Errors:
			if err != nil {
				return nil, err
			}
		case e := <-w.inner.Events:
			if e.Op&fsnotify.Chmod != 0 {
				continue
			}
			if isDir(e.Name) {
				// A new directory carries new files. The watcher reads it
				// from now on.
				if e.Op&fsnotify.Create != 0 && !skipDir(filepath.Base(e.Name)) {
					_ = w.inner.Add(e.Name)
				}
				continue
			}
			if !source(e.Name) {
				continue
			}
			files[e.Name] = true

			// Collect every event of one save.
			timer := time.NewTimer(Debounce)
			for waiting := true; waiting; {
				select {
				case <-ctx.Done():
					timer.Stop()
					return nil, ctx.Err()
				case more := <-w.inner.Events:
					if source(more.Name) && more.Op&fsnotify.Chmod == 0 {
						files[more.Name] = true
					}
					timer.Reset(Debounce)
				case <-timer.C:
					waiting = false
				}
			}
			out := make([]string, 0, len(files))
			for name := range files {
				out = append(out, name)
			}
			return out, nil
		}
	}
}

// isDir reports a path that holds a directory.
func isDir(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

// skipDir reports a directory that holds no source of the person.
func skipDir(name string) bool {
	switch name {
	case "node_modules", "vendor", "bin", "dist", "testdata":
		return true
	}
	return strings.HasPrefix(name, ".")
}

// source reports a file that the loop watches.
func source(path string) bool {
	if strings.HasSuffix(path, "zz_generated.go") || strings.HasSuffix(path, "zz_generated_client.go") {
		// A generated file follows its source, so it raises no second
		// rebuild.
		return false
	}
	switch strings.ToLower(filepath.Ext(path)) {
	case ".go", ".templ", ".css", ".js", ".sql", ".json":
		return true
	}
	return false
}
