package assets

import (
	"io/fs"
	"net/http"
	"strings"
)

// CacheControl is the header of a hashed asset. The name carries the content
// hash, so the file never changes and the browser can hold it for one year.
const CacheControl = "public, max-age=31536000, immutable"

// Handler serves the built assets of a manifest from a file system.
//
// It serves a file that the manifest names and nothing else, so a file that
// the output directory happens to hold never reaches the network. The
// application passes the embedded file system, so the binary carries the
// assets and needs no directory beside it.
//
//	r.Mount("/assets/", assets.Handler(dist, manifest))
func Handler(fsys fs.FS, m *Manifest) http.Handler {
	files := make(map[string]bool, m.Len())
	for _, name := range m.Names() {
		e, _ := m.Entry(name)
		files[e.File] = true
	}
	server := http.FileServerFS(fsys)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			http.Error(w, "the asset handler answers GET and HEAD", http.StatusMethodNotAllowed)
			return
		}
		name := strings.TrimPrefix(r.URL.Path, "/")
		if !files[name] {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Cache-Control", CacheControl)
		server.ServeHTTP(w, r)
	})
}
