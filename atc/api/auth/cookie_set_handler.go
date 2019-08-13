package auth

import (
	"context"
	"github.com/concourse/concourse/skymarshal/skyserver"
	"net/http"
	"strconv"
)

type CookieSetHandler struct {
	Handler http.Handler
}

func (handler CookieSetHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("Authorization") == "" {
		authCookie := ""
		for i := 0; i < skyserver.NumCookies; i++ {
			cookie, err := r.Cookie(AuthCookieName + strconv.Itoa(i))
			if err == nil {
				ctx := context.WithValue(r.Context(), CSRFRequiredKey, handler.isCSRFRequired(r))
				r = r.WithContext(ctx)
				authCookie += cookie.Value
			}
		}
		r.Header.Set("Authorization", authCookie)
	}

	handler.Handler.ServeHTTP(w, r)
}

// We don't validate CSRF token for GET requests
// since they are not changing the state
func (handler CookieSetHandler) isCSRFRequired(r *http.Request) bool {
	return (r.Method != http.MethodGet && r.Method != http.MethodHead && r.Method != http.MethodOptions)
}

func IsCSRFRequired(r *http.Request) bool {
	required, ok := r.Context().Value(CSRFRequiredKey).(bool)
	return ok && required
}
