package assets

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"path"
	"strings"
)

// HashLength is the number of hexadecimal characters of an asset hash. Twelve
// characters hold 48 bits, so a collision between two files of one application
// is not credible.
const HashLength = 12

// Hash returns the content hash of an asset. A changed file gives a different
// hash, so the hashed name breaks the cache of the browser.
func Hash(content []byte) string {
	sum := sha256.Sum256(content)
	return hex.EncodeToString(sum[:])[:HashLength]
}

// Integrity returns the subresource integrity value of an asset.
func Integrity(content []byte) string {
	sum := sha256.Sum256(content)
	return "sha256-" + base64.StdEncoding.EncodeToString(sum[:])
}

// HashedName puts the hash between the stem and the extension of a name.
//
//	HashedName("css/app.css", "a1b2c3d4e5f6") == "css/app.a1b2c3d4e5f6.css"
func HashedName(name, hash string) string {
	ext := path.Ext(name)
	return strings.TrimSuffix(name, ext) + "." + hash + ext
}
