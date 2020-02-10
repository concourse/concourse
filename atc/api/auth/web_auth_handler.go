package auth

import (
	"context"
	"net/http"

	"github.com/concourse/concourse/skymarshal/token"
)

//go:generate counterfeiter net/http.Handler

func NewResponseWrapper(w http.ResponseWriter, m token.Middleware) *responseWrapper {
	return &responseWrapper{w, m}
}

type responseWrapper struct {
	http.ResponseWriter
	token.Middleware
}

func (r *responseWrapper) WriteHeader(statusCode int) {

	// we need to unset cookies before writing the header
	if statusCode == http.StatusUnauthorized {
		r.Middleware.UnsetAuthToken(r.ResponseWriter)
		r.Middleware.UnsetCSRFToken(r.ResponseWriter)
	}

	r.ResponseWriter.WriteHeader(statusCode)
}

type WebAuthHandler struct {
	Handler    http.Handler
	Middleware token.Middleware
}

func (handler WebAuthHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {

	tokenString := handler.Middleware.GetAuthToken(r)
	if tokenString != "" {
		ctx := context.WithValue(r.Context(), CSRFRequiredKey, handler.isCSRFRequired(r))
		r = r.WithContext(ctx)

		if r.Header.Get("Authorization") == "" {
			r.Header.Set("Authorization", tokenString)
		}

		wrapper := NewResponseWrapper(w, handler.Middleware)
		handler.Handler.ServeHTTP(wrapper, r)
	} else {
		handler.Handler.ServeHTTP(w, r)
	}
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
