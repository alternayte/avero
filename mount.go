package avero

import (
	"io/fs"
	"path"

	"github.com/alternayte/avero/assets"
)

// MountAssets reads the manifest of the built assets and serves them at
// /assets/.
//
// root is the directory of the built output inside dist, such as
// "assets/dist". The manifest stands beside the files, so the caller states
// one path and not two. Mount strips the prefix, so the handler reads the
// path of the asset.
//
//	manifest, err := avero.MountAssets(r, dist, "assets/dist")
func MountAssets(r *Router, dist fs.FS, root string) (*Manifest, error) {
	manifest, err := assets.LoadManifest(dist, path.Join(root, "manifest.json"))
	if err != nil {
		return nil, err
	}
	files, err := fs.Sub(dist, root)
	if err != nil {
		return nil, err
	}
	r.Mount("/assets/", assets.Handler(files, manifest))
	return manifest, nil
}
