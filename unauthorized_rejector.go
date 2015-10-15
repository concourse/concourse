package auth

import (
	"fmt"
	"net/http"
)

type UnauthorizedRejector struct{}

func (UnauthorizedRejector) Unauthorized(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusUnauthorized)
	fmt.Fprintf(w, "not authorized")
}
