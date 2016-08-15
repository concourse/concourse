package auth

import "net/http"

func GetTeam(r *http.Request) (string, int, bool, bool) {
	teamName, namePresent := r.Context().Value(teamNameKey).(string)
	teamID, idPresent := r.Context().Value(teamIDKey).(int)
	isAdmin, adminPresent := r.Context().Value(isAdminKey).(bool)

	if !(namePresent && idPresent && adminPresent) {
		return "", 0, false, false
	}

	return teamName, teamID, isAdmin, true
}
