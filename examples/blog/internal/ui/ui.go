// Package ui holds the views of the application. It imports no feature
// package, so a view never depends on the code that calls it. See the SDD,
// S10.
package ui

import "github.com/alternayte/avero"

// manifest resolves the name of an asset to its hashed path. wire.go sets it
// one time at start.
var manifest *avero.Manifest

// SetManifest records the asset manifest of the application.
func SetManifest(m *avero.Manifest) { manifest = m }

// Asset returns the path of a built asset.
//
//	<link rel="stylesheet" href={ ui.Asset("app.css") }>
func Asset(name string) string { return manifest.Asset(name) }

// PostRow is one post that a page renders. A view takes plain values, so it
// needs no import of the feature that reads them.
type PostRow struct {
	// ID identifies the post.
	ID string
	// Title is the name that a person reads.
	Title string
	// Body is the text of the post.
	Body string
}
