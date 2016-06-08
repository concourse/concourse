package buildserver

import (
	"net/http"

	"github.com/concourse/atc"
	"github.com/concourse/atc/auth"
)

func getTeamName(r *http.Request) string {
	teamName, _, _, found := auth.GetTeam(r)
	if !found {
		teamName = atc.DefaultTeamName
	}

	return teamName
}
