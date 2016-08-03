package auth

import (
	"net/http"

	"github.com/gorilla/context"
)

type AuthorizationResponse string

const (
	Authorized   AuthorizationResponse = "authorized"
	Unauthorized AuthorizationResponse = "unauthorized"
	Forbidden    AuthorizationResponse = "forbidden"
)

func IsAuthorized(r *http.Request) (bool, AuthorizationResponse) {
	authenticated := IsAuthenticated(r)
	if !authenticated {
		return false, Unauthorized
	}

	authTeamName, ok := context.GetOk(r, teamNameKey)

	if !ok || r.URL.Query().Get(":team_name") != authTeamName {
		return false, Forbidden
	}

	return true, Authorized
}
