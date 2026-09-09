package assets

import (
	"regexp"
	"strings"
)

// cdnSpecifier matches an import that a CDN writes as an absolute path, such
// as /react@19.2.0/es2022/react.mjs or /react@^19.1.1/jsx-runtime?target=es2022.
var cdnSpecifier = regexp.MustCompile(`(from\s*|import\s*)"(/[^"]+)"`)

// buildTargets holds the directory that a CDN uses for one compile target. A
// path that starts with one carries no subpath of the package.
var buildTargets = map[string]bool{
	"es2015": true, "es2016": true, "es2017": true, "es2018": true,
	"es2019": true, "es2020": true, "es2021": true, "es2022": true,
	"es2023": true, "es2024": true, "esnext": true, "denonext": true, "node": true,
}

// vendorSpecifiers rewrites the imports of a module that a CDN wrote.
//
// A CDN keeps a peer dependency outside the bundle, and it names it with an
// absolute path of its own host, such as /react@19.2.0/es2022/react.mjs. That
// path reaches no file of the application, so the pin rewrites it to the name
// of the package, which the bundler resolves in the vendor directory.
//
// The rewrite is deterministic, so two runs give one file and the lock holds.
func vendorSpecifiers(body []byte) []byte {
	return cdnSpecifier.ReplaceAllFunc(body, func(match []byte) []byte {
		parts := cdnSpecifier.FindSubmatch(match)
		if len(parts) != 3 {
			return match
		}
		bare := bareSpecifier(string(parts[2]))
		if bare == "" {
			return match
		}
		return append(append([]byte(nil), parts[1]...), []byte(`"`+bare+`"`)...)
	})
}

// bareSpecifier returns the name of the package that an absolute CDN path
// names, with the subpath that the path carries. It returns the empty string
// for a path that names no package.
//
//	/react@19.2.0/es2022/react.mjs        react
//	/react@^19.1.1/jsx-runtime?target=x   react/jsx-runtime
//	/@tanstack/query-core@5.90.2/es2022/query-core.mjs   @tanstack/query-core
func bareSpecifier(path string) string {
	path = strings.TrimPrefix(path, "/")
	if i := strings.IndexAny(path, "?#"); i >= 0 {
		path = path[:i]
	}
	segments := strings.Split(path, "/")
	if len(segments) == 0 || segments[0] == "" {
		return ""
	}

	name := segments[0]
	rest := segments[1:]
	if strings.HasPrefix(name, "@") {
		if len(rest) == 0 {
			return ""
		}
		name += "/" + rest[0]
		rest = rest[1:]
	}
	at := strings.LastIndex(name, "@")
	if at <= 0 {
		// The path carries no version, so it names no package of a CDN.
		return ""
	}
	name = name[:at]

	if len(rest) > 0 && buildTargets[rest[0]] {
		return name
	}
	if len(rest) == 0 {
		return name
	}
	return name + "/" + strings.Join(rest, "/")
}
