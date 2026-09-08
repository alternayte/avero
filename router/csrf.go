package router

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"net/http"

	"github.com/alternayte/avero/config"
)

// The names that the CSRF middleware uses.
const (
	// CSRFCookieName is the cookie that carries the signed token.
	CSRFCookieName = "_avero_csrf"
	// CSRFFieldName is the form field that carries the token.
	CSRFFieldName = "_csrf"
	// CSRFHeaderName is the header that carries the token.
	CSRFHeaderName = "X-CSRF-Token"
	// StatusCSRF is the status of a CSRF fault. See the SDD, S10.
	StatusCSRF = 419
)

type csrfKey struct{}

// CSRF protects an unsafe method with a signed double-submit token.
//
// A safe method receives a token in a signed cookie. An unsafe method must
// send the same token back in the form field _csrf or in the X-CSRF-Token
// header. A request that does not match returns 419 and never reaches the
// handler.
//
// The token needs no server state, so it works for an anonymous visitor and
// for a signed-in one. auth-all owns the authentication session.
func CSRF(secret config.Secret, opts ...CookieOption) Middleware {
	sign := newSigner(secret, "csrf")
	return Middleware{Name: "csrf", Wrap: func(next Handler) Handler {
		return func(c *Ctx) (Response, error) {
			token, valid := readToken(c, sign)
			if !valid {
				token = newToken()
				http.SetCookie(c.Writer(), newCookie(CSRFCookieName, sign.sign([]byte(token)), opts))
			}
			c.setRequest(c.Request().WithContext(
				contextWithValue(c.Context(), csrfKey{}, token)))

			if isSafeMethod(c.Request().Method) {
				return next(c)
			}
			if !valid || !matches(c, token) {
				return Text(StatusCSRF, "the CSRF token is absent or wrong"), nil
			}
			return next(c)
		}
	}}
}

// CSRFToken returns the token of this request. A form embeds it in a hidden
// field named _csrf.
func (c *Ctx) CSRFToken() string {
	token, _ := c.Context().Value(csrfKey{}).(string)
	return token
}

// readToken returns the token of the cookie and reports whether the signature
// holds.
func readToken(c *Ctx, sign signer) (string, bool) {
	raw := readCookie(c, CSRFCookieName)
	if raw == "" {
		return "", false
	}
	value, ok := sign.verify(raw)
	if !ok {
		return "", false
	}
	return string(value), true
}

// matches reports whether the request carries the token of the cookie.
func matches(c *Ctx, token string) bool {
	given := c.Request().Header.Get(CSRFHeaderName)
	if given == "" {
		// ParseForm reads the body one time and keeps it in the request.
		if err := c.Request().ParseForm(); err == nil {
			given = c.Request().PostFormValue(CSRFFieldName)
		}
	}
	if given == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(given), []byte(token)) == 1
}

// isSafeMethod reports whether a method changes no state.
func isSafeMethod(method string) bool {
	switch method {
	case http.MethodGet, http.MethodHead, http.MethodOptions, http.MethodTrace:
		return true
	}
	return false
}

// newToken returns a random token.
func newToken() string {
	var b [32]byte
	_, _ = rand.Read(b[:])
	return base64.RawURLEncoding.EncodeToString(b[:])
}
