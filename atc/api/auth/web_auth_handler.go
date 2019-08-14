package auth

import (
	"context"
	"net/http"

	"github.com/concourse/concourse/skymarshal/token"
)

type WebAuthHandler struct {
	Handler http.Handler
}

func (handler WebAuthHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	tokenString := token.NewMiddleware(true).GetToken(r)
	if tokenString != "" {
		ctx := context.WithValue(r.Context(), CSRFRequiredKey, handler.isCSRFRequired(r))
		r = r.WithContext(ctx)

		if r.Header.Get("Authorization") == "" {
			r.Header.Set("Authorization", tokenString)
		}
	}

	handler.Handler.ServeHTTP(w, r)
}

// We don't validate CSRF token for GET requests
// since they are not changing the state
func (handler WebAuthHandler) isCSRFRequired(r *http.Request) bool {
	return (r.Method != http.MethodGet && r.Method != http.MethodHead && r.Method != http.MethodOptions)
}

func IsCSRFRequired(r *http.Request) bool {
	required, ok := r.Context().Value(CSRFRequiredKey).(bool)
	return ok && required
}
