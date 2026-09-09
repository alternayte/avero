package assets

import (
	"context"
	"fmt"
	"regexp"
	"strings"
)

// CDN is the address that resolves the name of a package to a bundled ES
// module. `avero js pin <package>` reads it, so a person names the package and
// no address.
const CDN = "https://esm.sh"

// bundlePath reads the module that a CDN answer exports.
var bundlePath = regexp.MustCompile(`(?m)^export \* from "(/[^"]+)";`)

// Resolve returns the address of the bundled module of one package.
//
// It asks the CDN for the package, and the answer names the file that holds
// the bundle with its resolved version. The pin records that address, so a
// second machine reads the same bytes.
//
//	Resolve(ctx, f, "zustand")               the newest version
//	Resolve(ctx, f, "zustand@5.0.15")        one version
//	Resolve(ctx, f, "react-dom/client")      one subpath
func Resolve(ctx context.Context, fetch Fetcher, spec string) (string, error) {
	spec = strings.TrimSpace(spec)
	if spec == "" {
		return "", fault("", "the pin names no package",
			"Run `avero js pin <package>`, such as `avero js pin zustand`")
	}
	query := fmt.Sprintf("%s/%s?bundle&target=es2022", CDN, spec)
	body, err := fetch.Fetch(ctx, query)
	if err != nil {
		return "", fault("", fmt.Sprintf("the package %q does not resolve at %s: %v", spec, CDN, err),
			"Prove the name of the package, or pass the address of the module after the name")
	}
	m := bundlePath.FindSubmatch(body)
	if m == nil {
		// The answer holds the module itself, so the query is the address.
		return query, nil
	}
	return CDN + string(m[1]), nil
}
