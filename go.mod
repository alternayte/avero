module github.com/alternayte/avero

go 1.26.2

// github.com/alternayte/drel is a direct dependency of the router package. It
// is absent here because drel does not publish yet: its module path names the
// repository github.com/alternayte/drel, which does not exist, and no tag
// carries the context transaction API. go.work supplies the local checkout.
//
// Add the require and run `go mod tidy` when drel publishes a version. See
// the note inside go.work.

require github.com/alternayte/auth-all v0.2.0

require (
	github.com/google/uuid v1.6.0 // indirect
	golang.org/x/crypto v0.55.0 // indirect
	golang.org/x/sys v0.47.0 // indirect
)
