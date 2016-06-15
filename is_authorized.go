package auth

import (
	"net/http"

	"github.com/concourse/atc"
	"github.com/gorilla/context"
)

func IsAuthorized(r *http.Request) bool {
	if IsAuthenticated(r) {
		authTeamName, ok := context.GetOk(r, teamNameKey)
		if !ok {
			authTeamName = atc.DefaultTeamName
		}

		return r.URL.Query().Get(":team_name") == authTeamName
	}

	return false
}
