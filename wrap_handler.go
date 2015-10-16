package auth

import (
	"net/http"

	"github.com/gorilla/context"
)

var authenticated = &struct{}{}

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
		context.Set(r, authenticated, true)
		h.handler.ServeHTTP(w, r)
	} else {
		h.rejector.Unauthorized(w, r)
	}
}

func IsAuthenticated(r *http.Request) bool {
	ugh, present := context.GetOk(r, authenticated)

	var isAuthenticated bool
	if present {
		isAuthenticated = ugh.(bool)
	}

	return isAuthenticated
}
