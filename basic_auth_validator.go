package auth

import (
	"net/http"

	"github.com/concourse/atc/db"

	"golang.org/x/crypto/bcrypt"
)

type BasicAuthValidator struct {
	TeamDBFactory db.TeamDBFactory
}

// IsAuthenticated
// basic authentication for login
func (validator BasicAuthValidator) IsAuthenticated(r *http.Request) bool {
	teamName := r.FormValue(":team_name")
	teamDB := validator.TeamDBFactory.GetTeamDB(teamName)
	team, found, err := teamDB.GetTeam()
	if err != nil || !found {
		return false
	}

	if team.BasicAuth == nil {
		return false
	}

	auth := r.Header.Get("Authorization")
	username, password, err := extractUsernameAndPassword(auth)
	if err != nil {
		return false
	}

	return validator.correctCredentials(
		team.BasicAuth.BasicAuthUsername, team.BasicAuth.BasicAuthPassword,
		username, password,
	)
}

func (validator BasicAuthValidator) correctCredentials(
	teamUsername string, teamPassword string,
	checkUsername string, checkPassword string,
) bool {
	err := bcrypt.CompareHashAndPassword([]byte(teamPassword), []byte(checkPassword))
	if err != nil {
		return false
	}
	return teamUsername == checkUsername
}
