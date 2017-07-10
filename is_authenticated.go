package auth

import "net/http"

func IsAuthenticated(r *http.Request) bool {
	isAuthenticated, _ := r.Context().Value(authenticated).(bool)
	return isAuthenticated
}
