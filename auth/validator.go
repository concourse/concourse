package auth

import "net/http"

//go:generate counterfeiter . Validator
type Validator interface {
	IsAuthenticated(*http.Request) bool
}
