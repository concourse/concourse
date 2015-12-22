package authserver

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/concourse/atc"
	"github.com/concourse/atc/auth"
)

const tokenDuration = 24 * time.Hour

func (s *Server) GetAuthToken(w http.ResponseWriter, r *http.Request) {
	logger := s.logger.Session("get-auth-token")

	authorization := r.Header.Get("Authorization")

	authSegs := strings.SplitN(authorization, " ", 2)
	if len(authSegs) != 2 {
		logger.Debug("malformed-authorization-header")
		w.WriteHeader(http.StatusBadRequest)
	}

	var token atc.AuthToken
	if strings.ToLower(authSegs[0]) == strings.ToLower(auth.TokenTypeBearer) {
		token.Type = authSegs[0]
		token.Value = authSegs[1]
	} else {
		team, err := s.db.GetTeamByName(atc.DefaultTeamName)
		if err != nil {
			logger.Error("get-team-by-name", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		tokenType, tokenValue, err := s.tokenGenerator.GenerateToken(time.Now().Add(tokenDuration), team.Name, team.ID)
		if err != nil {
			logger.Error("generate-token", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		token.Type = string(tokenType)
		token.Value = string(tokenValue)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(token)
}
