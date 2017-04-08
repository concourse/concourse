package authserver

import (
	"encoding/json"
	"net/http"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/atc"
)

// GetAccessToken returns an API token to be used to perform basic authorization for a CheckResource
func (s *Server) GetAccessToken(w http.ResponseWriter, r *http.Request) {
	logger := s.logger.Session("get-access-token")
	logger.Debug("getting-access-token")

	var token atc.AuthToken
	teamName := r.FormValue(":team_name")
	teamDB := s.teamDBFactory.GetTeamDB(teamName)
	_, found, err := teamDB.GetTeam()
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

	tokenType, tokenValue, err := s.accessTokenGenerator.GenerateToken()
	if err != nil {
		logger.Error("generate-access-token", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	//TODO: Save token to DB once it has been generated

	token.Type = string(tokenType)
	token.Value = string(tokenValue)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(token)
}
