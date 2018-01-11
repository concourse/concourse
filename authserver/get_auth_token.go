package authserver

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/atc/db"
	"github.com/concourse/skymarshal/auth"
	"github.com/concourse/skymarshal/provider"
)

func (s *Server) GetAuthToken(w http.ResponseWriter, r *http.Request) {

	logger := s.logger.Session("get-auth-token")
	logger.Debug("getting-auth-token")

	w.Header().Set("Content-Type", "application/json")

	teamName := r.FormValue("team_name")
	team, _, err := s.teamFactory.FindTeam(teamName)
	if err != nil {
		logger.Error("get-team-by-name", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if !s.basicAuthValidator.IsAuthenticated(r) {
		w.WriteHeader(http.StatusUnauthorized)
		fmt.Fprintf(w, "not-authorized")
		return
	}

	err = s.generateToken(logger, w, r, team)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
}

func (s *Server) generateToken(logger lager.Logger, w http.ResponseWriter, r *http.Request, team db.Team) error {
	var token provider.AuthToken

	csrfToken, err := s.csrfTokenGenerator.GenerateToken()
	if err != nil {
		logger.Error("generate-csrf-token", err)
	}

	tokenType, tokenValue, err := s.authTokenGenerator.GenerateToken(time.Now().Add(s.expire), team.Name(), team.Admin(), csrfToken)
	if err != nil {
		logger.Error("generate-auth-token", err)
		return err
	}

	token.Type = string(tokenType)
	token.Value = string(tokenValue)

	expiry := time.Now().Add(s.expire)

	authCookie := &http.Cookie{
		Name:     auth.AuthCookieName,
		Value:    fmt.Sprintf("%s %s", token.Type, token.Value),
		Path:     "/",
		Expires:  expiry,
		HttpOnly: true,
	}
	if s.isTLSEnabled {
		authCookie.Secure = true
	}
	// TODO: Add SameSite once Golang supports it
	// https://github.com/golang/go/issues/15867
	http.SetCookie(w, authCookie)

	w.Header().Set(auth.CSRFHeaderName, csrfToken)
	w.Header().Set("Content-Type", "application/json")
	err = json.NewEncoder(w).Encode(token)
	if err != nil {
		logger.Error("encode-auth-token", err)
		return err
	}
	return nil
}
