package auth

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/atc/db"
	"github.com/concourse/skymarshal/basicauth"
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
	v.logger.Info("IsAuthenticated")

	teamName := r.FormValue("team_name")
	team, found, err := v.teamFactory.FindTeam(teamName)
	if err != nil || !found {
		return false
	}

	v.logger.Info("IsAuthenticated: " + team.Name())

	_, configured := team.Auth()["noauth"]
	if configured {
		v.logger.Info("IsAuthenticated noauth is configured")
		return true
	}

	config, configured := team.Auth()["basicauth"]
	if !configured {
		v.logger.Info("IsAuthenticated basic auth is NOT configured")
		return false
	}

	header := r.Header.Get("Authorization")
	username, password, err := v.extractCredentials(header)
	if err != nil {
		return false
	}

	return v.verifyCredentials(config, username, password)
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

func (v basicAuthValidator) verifyCredentials(config *json.RawMessage, username string, password string) bool {
	provider := basicauth.BasicAuthTeamProvider{}

	authConfig, err := provider.UnmarshalConfig(config)
	if err != nil {
		v.logger.Info("verifyCredentials: " + err.Error())
		return false
	}

	p, ok := provider.ProviderConstructor(authConfig, username, password)
	if !ok {
		v.logger.Info("verifyCredentials: constructor fail")
		return false
	}

	valid, err := p.Verify(v.logger, nil)
	if err != nil {
		v.logger.Info("verifyCredentials: " + err.Error())
		return false
	}

	return valid
}
