package dev

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
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
//
// A goroutine drains the notifications without a pause, because a build takes
// seconds and the library loses an event that nothing reads. The goroutine
// collects the changes, and the loop reads them when it is free again.
type watcher struct {
	dir     string
	inner   *fsnotify.Watcher
	changes chan struct{}

	mu      sync.Mutex
	pending map[string]bool
}

// newWatcher opens a watcher on the tree of the application. It adds each
// directory, because fsnotify reports one directory and not a whole tree.
func newWatcher(dir string) (*watcher, error) {
	inner, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("avero dev: the watcher does not open: %w\n  → Raise the file limit of the shell, then run the command again", err)
	}
	w := &watcher{
		dir:     dir,
		inner:   inner,
		changes: make(chan struct{}, 1),
		pending: map[string]bool{},
	}
	if err := w.addTree(); err != nil {
		_ = inner.Close()
		return nil, err
	}
	go w.drain()
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

// drain reads every notification and records the source files.
//
// It never blocks on the loop, so no notification is lost while a build runs.
func (w *watcher) drain() {
	for {
		e, ok := <-w.inner.Events
		if !ok {
			return
		}
		if e.Op == fsnotify.Chmod {
			// A change of the mode alone is no change of the content. A
			// write carries the mode with it on macOS, which reports
			// WRITE|CHMOD, so the test is an equality and not a mask.
			continue
		}
		if isDir(e.Name) {
			// A new directory carries new files. The watcher reads it from
			// now on.
			if e.Op&fsnotify.Create != 0 && !skipDir(filepath.Base(e.Name)) {
				_ = w.inner.Add(e.Name)
			}
			continue
		}
		if !source(e.Name) {
			continue
		}
		w.mu.Lock()
		w.pending[e.Name] = true
		w.mu.Unlock()
		select {
		case w.changes <- struct{}{}:
		default:
		}
	}
}

// take returns the files that changed, and clears the set.
func (w *watcher) take() []string {
	w.mu.Lock()
	defer w.mu.Unlock()
	if len(w.pending) == 0 {
		return nil
	}
	out := make([]string, 0, len(w.pending))
	for name := range w.pending {
		out = append(out, name)
	}
	w.pending = map[string]bool{}
	sort.Strings(out)
	return out
}

// next returns the source files of one change. It waits for the debounce, so
// one save of an editor gives one rebuild.
func (w *watcher) next(ctx context.Context) ([]string, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-w.changes:
	}
	// Let the rest of one save arrive.
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(Debounce):
	}
	return w.take(), nil
}

// sweep returns the source files that changed after a time.
//
// The loop calls it after each rebuild. A notification that arrives while the
// build runs can reach a library buffer that nothing reads, so the sweep is
// the proof that no change is lost. It reads the tree one time, which costs a
// few milliseconds.
func (w *watcher) sweep(since time.Time) []string {
	var out []string
	_ = filepath.WalkDir(w.dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if path != w.dir && skipDir(d.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		if !source(path) {
			return nil
		}
		info, statErr := d.Info()
		if statErr != nil || info.ModTime().Before(since) {
			return nil
		}
		out = append(out, path)
		return nil
	})
	sort.Strings(out)
	return out
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
	case ".go", ".templ", ".css", ".js", ".jsx", ".ts", ".tsx", ".sql", ".json":
		return true
	}
	return false
}
