package auth

import (
	"net/http"

	"github.com/gorilla/context"
)

var authenticated = &struct{}{}

func WrapHandler(
	handler http.Handler,
	validator Validator,
) http.Handler {
	return authHandler{
		handler:   handler,
		validator: validator,
	}
}

type authHandler struct {
	handler   http.Handler
	validator Validator
}

func (h authHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	context.Set(r, authenticated, h.validator.IsAuthenticated(r))
	h.handler.ServeHTTP(w, r)
}
