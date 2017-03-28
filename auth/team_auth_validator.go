package auth

import (
	"net/http"

	"github.com/concourse/atc/auth/provider"
	"github.com/concourse/atc/db"
)

type teamAuthValidator struct {
	teamDBFactory db.TeamDBFactory
	jwtValidator  Validator
}

func NewTeamAuthValidator(
	teamDBFactory db.TeamDBFactory,
	jwtValidator Validator,
) Validator {
	return &teamAuthValidator{
		teamDBFactory: teamDBFactory,
		jwtValidator:  jwtValidator,
	}
}

func (v teamAuthValidator) IsAuthenticated(r *http.Request) bool {
	teamName := r.FormValue(":team_name")
	teamDB := v.teamDBFactory.GetTeamDB(teamName)
	team, found, err := teamDB.GetTeam()
	if err != nil || !found {
		return false
	}

	if !isAuthConfigured(team.Team) {
		return true
	}

	if team.BasicAuth != nil && NewBasicAuthValidator(team).IsAuthenticated(r) {
		return true
	}

	return v.jwtValidator.IsAuthenticated(r)

}

func isAuthConfigured(t db.Team) bool {
	if t.BasicAuth != nil {
		return true
	}
	for _, p := range provider.GetProviders() {
		if p.ProviderConfigured(t) {
			return true
		}
	}
	return false
}
