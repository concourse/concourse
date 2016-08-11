package auth

import "net/http"

type AuthorizationResponse string

const (
	Authorized   AuthorizationResponse = "authorized"
	Unauthorized AuthorizationResponse = "unauthorized"
	Forbidden    AuthorizationResponse = "forbidden"
)

func IsAuthorized(r *http.Request) bool {
	authTeamName, _, _, found := GetTeam(r)

	if found && r.URL.Query().Get(":team_name") == authTeamName {
		return true
	}

	return false
}
