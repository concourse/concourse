package auth

import (
	"net/http"

	"github.com/concourse/atc/db"
)

type getTokenValidator struct {
	teamFactory db.TeamFactory
}

func NewGetTokenValidator(
	teamFactory db.TeamFactory,
) Validator {
	return &getTokenValidator{
		teamFactory: teamFactory,
	}
}

func (v getTokenValidator) IsAuthenticated(r *http.Request) bool {
	teamName := r.FormValue(":team_name")
	team, found, err := v.teamFactory.FindTeam(teamName)
	if err != nil || !found {
		return false
	}

	if !isAuthConfigured(team) {
		return true
	}

	if team.BasicAuth != nil && NewBasicAuthValidator(team).IsAuthenticated(r) {
		return true
	}

	return false
}
