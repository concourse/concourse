package auth

import "net/http"

func IsAuthorized(r *http.Request) bool {
	authTeam, authTeamFound := GetTeam(r)

	if authTeamFound && authTeam.IsAuthorized(r.URL.Query().Get(":team_name")) {
		return true
	}

	return false
}
