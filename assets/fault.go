// Package assets holds the asset pipeline, S11.
//
// The pipeline has three tiers. Tier 0 is the default and needs no Node.js: it
// bundles with esbuild as a Go library, resolves relative paths and vendored
// paths only, and drives the Tailwind standalone binary. Tier 1 adds a
// package.json, so esbuild resolves a bare specifier against node_modules.
// Tier 2 runs an external command that writes the same output.
//
// Every tier writes assets/dist and a manifest with one shape. A page reads
// the manifest with Asset, and the binary serves the files from an embedded
// file system. See the SDD, S11 and DX-9.
package assets

import (
	"fmt"
	"strings"
)

// Fault is one asset fault. It states what went wrong and states the repair in
// one sentence. It names a file when a file holds the fault. See DX-6 and
// DX-7.
type Fault struct {
	// File is the file that holds the fault. It is empty when no file
	// applies.
	File string `json:"file"`
	// Line is the line that holds the fault. It is zero when no line
	// applies.
	Line int `json:"line"`
	// Column is the column that holds the fault. It is zero when no column
	// applies.
	Column int `json:"column"`
	// Message states the fault.
	Message string `json:"message"`
	// Repair states what to do. It is one sentence.
	Repair string `json:"repair"`
}

// Error returns the position, the message and the repair.
func (f *Fault) Error() string {
	var b strings.Builder
	b.WriteString("assets: ")
	if f.File != "" {
		b.WriteString(f.File)
		if f.Line > 0 {
			fmt.Fprintf(&b, ":%d", f.Line)
			if f.Column > 0 {
				fmt.Fprintf(&b, ":%d", f.Column)
			}
		}
		b.WriteString(": ")
	}
	b.WriteString(f.Message)
	b.WriteString("\n  → ")
	b.WriteString(f.Repair)
	return b.String()
}

// Faults holds every fault that one build found. The build reports every
// fault, not only the first.
type Faults struct {
	Faults []*Fault `json:"faults"`
}

// Error returns a count and one entry for each fault.
func (l *Faults) Error() string {
	var b strings.Builder
	if len(l.Faults) == 1 {
		b.WriteString("assets: 1 fault\n\n")
	} else {
		fmt.Fprintf(&b, "assets: %d faults\n\n", len(l.Faults))
	}
	for i, f := range l.Faults {
		if i > 0 {
			b.WriteString("\n")
		}
		b.WriteString(f.Error())
		b.WriteString("\n")
	}
	return b.String()
}

// Unwrap returns each fault so that errors.As reaches a single one.
func (l *Faults) Unwrap() []error {
	out := make([]error, len(l.Faults))
	for i, f := range l.Faults {
		out[i] = f
	}
	return out
}

// fault returns one fault as an error.
func fault(file, message, repair string) error {
	return &Fault{File: file, Message: message, Repair: repair}
}
