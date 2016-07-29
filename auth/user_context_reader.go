package auth

import "net/http"

//go:generate counterfeiter . UserContextReader

type UserContextReader interface {
	GetTeam(r *http.Request) (string, int, bool, bool)
	GetSystem(r *http.Request) (bool, bool)
}
