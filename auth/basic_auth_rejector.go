package auth

import (
	"fmt"
	"net/http"
)

type BasicAuthRejector struct{}

func (BasicAuthRejector) Unauthorized(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("WWW-Authenticate", `Basic realm="Restricted"`)
	w.WriteHeader(http.StatusUnauthorized)
	fmt.Fprintf(w, "not authorized")
}

func (BasicAuthRejector) Forbidden(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusForbidden)
	fmt.Fprintf(w, "forbidden")
}
