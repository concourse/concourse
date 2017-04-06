package teamserver

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/concourse/atc"
	"github.com/concourse/atc/api/present"
)

func (s *Server) ListTeams(w http.ResponseWriter, r *http.Request) {
	hLog := s.logger.Session("list-teams")

	savedTeams, err := s.teamsDB.GetTeams()
	if err != nil {
		hLog.Error("failed-to-get-teams", errors.New("sorry"))
		w.WriteHeader(http.StatusInternalServerError)
	}

	presentedTeams := make([]atc.Team, len(savedTeams))
	for i, savedTeam := range savedTeams {
		presentedTeams[i] = present.SavedTeam(savedTeam)
	}

	json.NewEncoder(w).Encode(presentedTeams)
}
