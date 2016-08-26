package auth

import (
	"fmt"
	"net/http"
)

func IsAdmin(r *http.Request) bool {
	isAdmin, present := r.Context().Value(isAdminKey).(bool)
	fmt.Printf("isAdmin %t and preset %t", isAdmin, present)
	return present && isAdmin
}
