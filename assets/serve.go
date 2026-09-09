package assets

import (
	"bytes"
	"io/fs"
	"net/http"
	"path"
	"regexp"
	"strings"
	"time"
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

// IndexFile is the document that a single page application serves for a path
// that it owns.
const IndexFile = "index.html"

// SPA serves the build of a single page application from a file system.
//
// A request for a file that the build holds answers that file. A hashed name
// answers with the cache of one year, because the name changes with the
// content. Any other path answers the index document, so a deep link and a
// reload reach the front end.
//
// The application embeds the output directory, so one binary holds the server
// and the front end.
//
//	//go:embed all:assets/dist
//	var dist embed.FS
//
//	files, _ := fs.Sub(dist, "assets/dist")
//	r.Mount("/", assets.SPA(files))
func SPA(fsys fs.FS) http.Handler {
	server := http.FileServerFS(fsys)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			http.Error(w, "the front end answers GET and HEAD", http.StatusMethodNotAllowed)
			return
		}
		name := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
		if name == "" || name == "." {
			name = IndexFile
		}
		info, err := fs.Stat(fsys, name)
		if err != nil || info.IsDir() || name == IndexFile {
			// The front end owns the path. It reads the path itself. The
			// index answers here and not through the file server, because a
			// file server answers a redirect for an index.
			serveIndex(w, r, fsys)
			return
		}
		if hashed(name) {
			w.Header().Set("Cache-Control", CacheControl)
		} else {
			w.Header().Set("Cache-Control", "no-cache")
		}
		r.URL.Path = "/" + name
		server.ServeHTTP(w, r)
	})
}

// serveIndex answers the index document of the build.
func serveIndex(w http.ResponseWriter, r *http.Request, fsys fs.FS) {
	body, err := fs.ReadFile(fsys, IndexFile)
	if err != nil {
		http.Error(w, "the build holds no "+IndexFile+
			"\n  → Run `avero build`, which writes the front end into the output directory",
			http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	http.ServeContent(w, r, IndexFile, time.Time{}, bytes.NewReader(body))
}

// hashedName matches a file that carries a content hash, such as
// index-C1p2Q3.js. A bundler writes such a name, and the content of the file
// never changes.
var hashedName = regexp.MustCompile(`-[A-Za-z0-9_-]{8,}\.[a-z0-9]+$`)

// hashed reports a file name that carries a content hash.
func hashed(name string) bool {
	if hashedName.MatchString(name) {
		return true
	}
	// The pipeline of Avero writes app.<hash>.css.
	parts := strings.Split(path.Base(name), ".")
	if len(parts) < 3 {
		return false
	}
	hash := parts[len(parts)-2]
	if len(hash) != HashLength {
		return false
	}
	for _, r := range hash {
		if !strings.ContainsRune("0123456789abcdef", r) {
			return false
		}
	}
	return true
}
