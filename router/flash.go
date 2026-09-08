package router

import (
	"context"
	"encoding/json"
	"net/http"
)

// FlashCookieName is the cookie that carries the toasts across a redirect.
const FlashCookieName = "_avero_flash"

// Flash carries a Toast from one request to the next.
//
// It reads the toasts of the previous request into the context and clears the
// cookie. After the handler returns, it writes the toasts into the cookie only
// when the response is a redirect. Any other response shows the toasts itself,
// so it needs no cookie.
func Flash(secret string, opts ...CookieOption) Middleware {
	sign := newSigner(secret, "flash")
	return Middleware{Name: "flash", Wrap: func(next Handler) Handler {
		return func(c *Ctx) (Response, error) {
			carried := readFlash(c, sign)
			if len(carried) > 0 {
				c.toasts = append(c.toasts, carried...)
				// The toasts are read one time. Clear the cookie now, so that
				// a handler that panics does not repeat them.
				http.SetCookie(c.Writer(), clearCookie(opts))
			}

			res, err := next(c)
			if err != nil {
				return res, err
			}
			if isRedirect(statusOf(res)) && len(c.toasts) > 0 {
				if body, mErr := json.Marshal(c.toasts); mErr == nil {
					http.SetCookie(c.Writer(), newCookie(FlashCookieName, sign.sign(body), opts))
				}
			}
			return res, nil
		}
	}}
}

// readFlash returns the toasts that the previous response wrote.
func readFlash(c *Ctx, sign signer) []Toast {
	raw := readCookie(c, FlashCookieName)
	if raw == "" {
		return nil
	}
	body, ok := sign.verify(raw)
	if !ok {
		return nil
	}
	var toasts []Toast
	if err := json.Unmarshal(body, &toasts); err != nil {
		return nil
	}
	return toasts
}

// clearCookie returns a cookie that removes the flash.
func clearCookie(opts []CookieOption) *http.Cookie {
	c := newCookie(FlashCookieName, "", opts)
	c.MaxAge = -1
	return c
}

// isRedirect reports whether a status sends the client to another page.
func isRedirect(code int) bool { return code >= 300 && code <= 399 }

// contextWithValue keeps the context helpers of this package in one place.
func contextWithValue(ctx context.Context, key, value any) context.Context {
	return context.WithValue(ctx, key, value)
}
