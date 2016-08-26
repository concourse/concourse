package auth

import "net/http"

func IsAdmin(r *http.Request) bool {
	isAdmin, present := r.Context().Value(isAdminKey).(bool)
	return present && isAdmin
}
