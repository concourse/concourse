package auth

import (
	"net/http"

	"github.com/gorilla/context"
)

func IsAuthenticated(r *http.Request) bool {
	ugh, present := context.GetOk(r, authenticated)

	var isAuthenticated bool
	if present {
		isAuthenticated = ugh.(bool)
	}

	return isAuthenticated
}
