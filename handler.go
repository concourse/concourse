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
		h.Validator.Unauthorized(w, r)
	}
}

type WebHandler struct {
	Validator Validator
	Handler   http.Handler
}

func (h WebHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if h.Validator.IsAuthenticated(r) {
		h.Handler.ServeHTTP(w, r)
	} else {
		h.Validator.Unauthorized(w, r)
	}
}
