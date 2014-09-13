package auth

import "net/http"

type Handler struct {
	Validator Validator
	Handler   http.Handler
}

const CookieName = "ATC-Authorization"

func (h Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if h.Validator.IsAuthenticated(w, r) {
		h.Handler.ServeHTTP(w, r)
	} else {
		h.Unauthorized(w)
	}
}

func (h Handler) Unauthorized(w http.ResponseWriter) {
	w.Header().Set("WWW-Authenticate", `Basic realm="Restricted"`)
	w.WriteHeader(http.StatusUnauthorized)
}
