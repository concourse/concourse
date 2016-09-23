package authserver

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/atc"
	"github.com/concourse/atc/auth"
)

const CookieName = "ATC-Authorization"

func (s *Server) GetAuthToken(w http.ResponseWriter, r *http.Request) {
	logger := s.logger.Session("get-auth-token")
	logger.Debug("getting-auth-token")

	authorization := r.Header.Get("Authorization")

	authSegs := strings.SplitN(authorization, " ", 2)
	var token atc.AuthToken
	if strings.ToLower(authSegs[0]) == strings.ToLower(auth.TokenTypeBearer) {
		token.Type = authSegs[0]
		token.Value = authSegs[1]
	} else {
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

		tokenType, tokenValue, err := s.tokenGenerator.GenerateToken(time.Now().Add(s.expire), team.Name, team.ID, team.Admin)
		if err != nil {
			logger.Error("generate-token", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		token.Type = string(tokenType)
		token.Value = string(tokenValue)

		http.SetCookie(w, &http.Cookie{
			Name:    CookieName,
			Value:   fmt.Sprintf("%s %s", token.Type, token.Value),
			Path:    "/",
			Expires: time.Now().Add(s.expire),
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(token)
}
