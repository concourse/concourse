package auth

import "net/http"

func IsSystem(r *http.Request) bool {
	isSystem, present := r.Context().Value(isSystemKey).(bool)
	return present && isSystem
}
