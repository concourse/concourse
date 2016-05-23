package web

import "net/http"

//go:generate counterfeiter . HTTPHandlerWithError

type HTTPHandlerWithError interface {
	ServeHTTP(w http.ResponseWriter, r *http.Request) error
}
