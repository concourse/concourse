package auth

import "net/http"

//go:generate counterfeiter . UserContextReader

type UserContextReader interface {
	GetTeam(r *http.Request) (string, bool, bool)
	GetSystem(r *http.Request) (bool, bool)
	GetCSRFToken(r *http.Request) (string, bool)
}
