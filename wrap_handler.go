package auth

import "net/http"

func WrapHandler(
	handler http.Handler,
	validator Validator,
	rejector Rejector,
) http.Handler {
	return authHandler{
		handler:   handler,
		validator: validator,
		rejector:  rejector,
	}
}

type authHandler struct {
	handler   http.Handler
	validator Validator
	rejector  Rejector
}

func (h authHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if h.validator.IsAuthenticated(r) {
		h.handler.ServeHTTP(w, r)
	} else {
		h.rejector.Unauthorized(w, r)
	}
}
