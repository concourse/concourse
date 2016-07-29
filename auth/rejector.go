package auth

import "net/http"

//go:generate counterfeiter . Rejector
type Rejector interface {
	Unauthorized(http.ResponseWriter, *http.Request)
	Forbidden(http.ResponseWriter, *http.Request)
}
