package auth

import (
	"net/http"

	"github.com/concourse/atc/db"

	"golang.org/x/crypto/bcrypt"
)

type basicAuthValidator struct {
	team db.Team
}

func NewBasicAuthValidator(team db.Team) Validator {
	return basicAuthValidator{
		team: team,
	}
}

func (v basicAuthValidator) IsAuthenticated(r *http.Request) bool {
	auth := r.Header.Get("Authorization")
	username, password, err := extractUsernameAndPassword(auth)
	if err != nil {
		return false
	}

	return v.correctCredentials(
		v.team.BasicAuth().BasicAuthUsername, v.team.BasicAuth().BasicAuthPassword,
		username, password,
	)
}

func (v basicAuthValidator) correctCredentials(
	teamUsername string, teamPassword string,
	checkUsername string, checkPassword string,
) bool {
	err := bcrypt.CompareHashAndPassword([]byte(teamPassword), []byte(checkPassword))
	if err != nil {
		return false
	}
	return teamUsername == checkUsername
}
