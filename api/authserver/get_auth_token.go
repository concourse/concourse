package authserver

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/atc"
	"github.com/concourse/atc/auth"
)

func (s *Server) GetAuthToken(w http.ResponseWriter, r *http.Request) {
	logger := s.logger.Session("get-auth-token")
	logger.Debug("getting-auth-token")

	var token atc.AuthToken
	teamName := r.FormValue(":team_name")
	teamDB := s.teamDBFactory.GetTeamDB(teamName)
	team, found, err := teamDB.GetTeam()
	if err != nil {
		logger.Error("get-team-by-name", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if !found {
		logger.Info("cannot-find-team-by-name", lager.Data{
			"teamName": teamName,
		})
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	csrfToken, err := s.csrfTokenGenerator.GenerateToken()
	if err != nil {
		logger.Error("generate-csrf-token", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	tokenType, tokenValue, err := s.authTokenGenerator.GenerateToken(time.Now().Add(s.expire), team.Name, team.Admin, csrfToken)
	if err != nil {
		logger.Error("generate-auth-token", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	token.Type = string(tokenType)
	token.Value = string(tokenValue)

	expiry := time.Now().Add(s.expire)

	csrfCookie := &http.Cookie{
		Name:    auth.CSRFCookieName,
		Value:   csrfToken,
		Path:    "/",
		Expires: expiry,
	}
	if s.isTLSEnabled {
		csrfCookie.Secure = true
	}
	// TODO: Add SameSite once Golang supports it
	// https://github.com/golang/go/issues/15867
	http.SetCookie(w, csrfCookie)

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
	json.NewEncoder(w).Encode(token)
}
