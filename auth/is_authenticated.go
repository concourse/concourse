package auth

import "net/http"

func IsAuthenticated(r *http.Request) bool {
	isAuthenticated, present := r.Context().Value(authenticated).(bool)
	return present && isAuthenticated
}
