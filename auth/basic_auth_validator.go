package auth

import (
	"net/http"

	"github.com/concourse/atc"

	"golang.org/x/crypto/bcrypt"
)

type BasicAuthValidator struct {
	DB AuthDB
}

// IsAuthenticated
// basic authentication for login
func (validator BasicAuthValidator) IsAuthenticated(r *http.Request) bool {
	auth := r.Header.Get("Authorization")

	username, password, err := extractUsernameAndPassword(auth)
	if err != nil {
		return false
	}

	team, err := validator.DB.GetTeamByName(atc.DefaultTeamName)
	if err != nil {
		return false
	}

	return validator.correctCredentials(
		team.BasicAuthUsername, team.BasicAuthPassword,
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
