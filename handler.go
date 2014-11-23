package auth

import "net/http"

type Handler struct {
	Validator Validator
	Handler   http.Handler
}

func (h Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if h.Validator.IsAuthenticated(r) {
		h.Handler.ServeHTTP(w, r)
	} else {
		Unauthorized(w)
	}
}

func Unauthorized(w http.ResponseWriter) {
	w.Header().Set("WWW-Authenticate", `Basic realm="Restricted"`)
	w.WriteHeader(http.StatusUnauthorized)
}
