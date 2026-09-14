// The CLI stands in a module of its own, so an application that imports the
// host carries no bundler and no file watcher.
//
//	go install github.com/alternayte/avero/cli/cmd/avero@latest
//
// go.work in the repository joins the two modules, so a change to the host
// reaches the CLI with no release and no replace directive.
module github.com/alternayte/avero/cli

go 1.26.2

require (
	// The host. The CLI scaffolds an application that imports it, and it
	// reads the manifest and the description of the API that the host writes.
	//
	// A require must name a tag that exists: go reads the go.mod of that
	// version even in the workspace. The release raises this line after it
	// tags the host, and the tag of this module follows. See the release
	// steps in the SDD, section 13.
	github.com/alternayte/avero v0.5.0
	// drel applies the migrations that `avero migrate` runs.
	github.com/alternayte/drel v0.7.1
	// esbuild is the bundler of the asset pipeline. S11 names it: "Bundle
	// with esbuild as a Go library." It runs at build time only. No request
	// path calls it.
	github.com/evanw/esbuild v0.28.2
	// fsnotify carries the file notifications of the development loop. S15
	// states "do not write a new watcher", so the loop reads the
	// notifications of the operating system through this library. `avero dev`
	// uses it, and no application imports it.
	github.com/fsnotify/fsnotify v1.10.1
)

require (
	github.com/antlr4-go/antlr/v4 v4.13.0 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/coder/websocket v1.8.12 // indirect
	github.com/dustin/go-humanize v1.0.1 // indirect
	github.com/go-logr/logr v1.4.3 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/jackc/pgpassfile v1.0.0 // indirect
	github.com/jackc/pgservicefile v0.0.0-20240606120523-5a60cdf6a761 // indirect
	github.com/jackc/pgx/v5 v5.10.0 // indirect
	github.com/jackc/puddle/v2 v2.2.2 // indirect
	github.com/mattn/go-isatty v0.0.24 // indirect
	github.com/ncruces/go-strftime v1.0.0 // indirect
	github.com/remyoudompheng/bigfft v0.0.0-20230129092748-24d4a6f8daec // indirect
	github.com/tursodatabase/libsql-client-go v0.0.0-20260528064733-9d5d30a29a60 // indirect
	go.opentelemetry.io/auto/sdk v1.2.1 // indirect
	go.opentelemetry.io/otel v1.41.0 // indirect
	go.opentelemetry.io/otel/metric v1.41.0 // indirect
	go.opentelemetry.io/otel/trace v1.41.0 // indirect
	golang.org/x/exp v0.0.0-20240325151524-a685a6edb6d8 // indirect
	golang.org/x/sync v0.22.0 // indirect
	golang.org/x/sys v0.47.0 // indirect
	golang.org/x/text v0.41.0 // indirect
	modernc.org/libc v1.74.4 // indirect
	modernc.org/mathutil v1.7.1 // indirect
	modernc.org/memory v1.11.0 // indirect
	modernc.org/sqlite v1.57.0 // indirect
)
