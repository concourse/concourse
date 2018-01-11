package auth

import (
	"encoding/base64"
	"errors"
	"net/http"
	"strings"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/atc/db"
	"github.com/concourse/skymarshal/provider"

	"golang.org/x/crypto/bcrypt"
)

var ErrUnparsableHeader = errors.New("cannot parse 'Authorization' header")

type basicAuthValidator struct {
	logger      lager.Logger
	teamFactory db.TeamFactory
}

func NewBasicAuthValidator(logger lager.Logger, teamFactory db.TeamFactory) Validator {
	return basicAuthValidator{
		logger:      logger,
		teamFactory: teamFactory,
	}
}

func (v basicAuthValidator) IsAuthenticated(r *http.Request) bool {

	teamName := r.FormValue("team_name")
	team, found, err := v.teamFactory.FindTeam(teamName)
	if err != nil || !found {
		return false
	}

	if !v.IsAuthConfigured(team) {
		return true
	}

	if team.BasicAuth() == nil {
		return false
	}

	header := r.Header.Get("Authorization")
	username, password, err := v.extractCredentials(header)
	if err != nil {
		return false
	}

	return v.verifyCredentials(
		team.BasicAuth().BasicAuthUsername,
		team.BasicAuth().BasicAuthPassword,
		username,
		password,
	)
}

func (v basicAuthValidator) IsAuthConfigured(team db.Team) bool {
	if team.BasicAuth() != nil {
		return true
	}

	for name := range provider.GetProviders() {
		_, configured := team.Auth()[name]
		if configured {
			return true
		}
	}

	return false
}

func (v basicAuthValidator) extractCredentials(header string) (string, string, error) {
	if !strings.HasPrefix(strings.ToUpper(header), "BASIC ") {
		return "", "", ErrUnparsableHeader
	}

	credentials, err := base64.StdEncoding.DecodeString(header[6:])
	if err != nil {
		return "", "", ErrUnparsableHeader
	}

	parts := strings.Split(string(credentials), ":")
	if len(parts) != 2 {
		return "", "", ErrUnparsableHeader
	}

	return parts[0], parts[1], nil
}

func (v basicAuthValidator) verifyCredentials(
	teamUsername string,
	teamPassword string,
	checkUsername string,
	checkPassword string,
) bool {
	err := bcrypt.CompareHashAndPassword([]byte(teamPassword), []byte(checkPassword))
	if err != nil {
		v.logger.Error("verify", err)
		return false
	}
	return teamUsername == checkUsername
}
