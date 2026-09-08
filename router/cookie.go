package router

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"net/http"
	"strings"
)

// MinSecretLength is the shortest secret that Avero accepts. A short secret
// makes a signature guessable.
const MinSecretLength = 32

// signer signs and verifies a cookie value with HMAC-SHA256.
//
// Avero owns the CSRF token and the flash. auth-all owns the authentication
// session. See the SDD, S4.
type signer struct{ key []byte }

// newSigner returns a signer. It panics on a secret that is too short, because
// a weak secret is a wiring fault that must stop the boot. See DX-8.
func newSigner(secret, use string) signer {
	if len(secret) < MinSecretLength {
		panic(fmt.Sprintf(
			"router: the %s secret is %d bytes, and it must be at least %d\n"+
				"  → Set a secret of at least %d bytes, for example the output of `openssl rand -hex 32`",
			use, len(secret), MinSecretLength, MinSecretLength))
	}
	sum := sha256.Sum256([]byte(secret + "|" + use))
	return signer{key: sum[:]}
}

// sign returns the value and its signature, joined by a dot.
func (s signer) sign(value []byte) string {
	mac := hmac.New(sha256.New, s.key)
	mac.Write(value)
	enc := base64.RawURLEncoding
	return enc.EncodeToString(value) + "." + enc.EncodeToString(mac.Sum(nil))
}

// verify returns the value when the signature holds.
func (s signer) verify(raw string) ([]byte, bool) {
	value, sig, ok := strings.Cut(raw, ".")
	if !ok {
		return nil, false
	}
	enc := base64.RawURLEncoding
	decoded, err := enc.DecodeString(value)
	if err != nil {
		return nil, false
	}
	given, err := enc.DecodeString(sig)
	if err != nil {
		return nil, false
	}
	mac := hmac.New(sha256.New, s.key)
	mac.Write(decoded)
	if subtle.ConstantTimeCompare(given, mac.Sum(nil)) != 1 {
		return nil, false
	}
	return decoded, true
}

// CookieOption configures a cookie that Avero sets.
type CookieOption func(*http.Cookie)

// Secure marks a cookie so that a browser sends it over TLS only. Set it in
// every environment except local development.
func Secure() CookieOption {
	return func(c *http.Cookie) { c.Secure = true }
}

// CookiePath sets the path of a cookie. The default is the root.
func CookiePath(path string) CookieOption {
	return func(c *http.Cookie) { c.Path = path }
}

// CookieDomain sets the domain of a cookie.
func CookieDomain(domain string) CookieOption {
	return func(c *http.Cookie) { c.Domain = domain }
}

// newCookie builds a cookie with the Avero defaults.
func newCookie(name, value string, opts []CookieOption) *http.Cookie {
	c := &http.Cookie{
		Name:     name,
		Value:    value,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// readCookie returns the value of a cookie.
func readCookie(c *Ctx, name string) string {
	cookie, err := c.Request().Cookie(name)
	if err != nil {
		return ""
	}
	return cookie.Value
}
