package auth

import "net/http"

type AuthorizationResponse string

const (
	Authorized   AuthorizationResponse = "authorized"
	Unauthorized AuthorizationResponse = "unauthorized"
	Forbidden    AuthorizationResponse = "forbidden"
)

func IsAuthorized(r *http.Request) bool {
	authTeam, authTeamFound := GetTeam(r)

	if authTeamFound && authTeam.IsAuthorized(r.URL.Query().Get(":team_name")) {
		return true
	}

	return false
}
