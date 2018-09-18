package auth

import (
	"fmt"
	"net/http"
)

//go:generate counterfeiter . Rejector
type Rejector interface {
	Unauthorized(http.ResponseWriter, *http.Request)
	Forbidden(http.ResponseWriter, *http.Request)
}

type UnauthorizedRejector struct{}

func (UnauthorizedRejector) Unauthorized(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusUnauthorized)
	fmt.Fprintf(w, "not authorized")
}

func (UnauthorizedRejector) Forbidden(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusForbidden)
	fmt.Fprintf(w, "forbidden")
}
