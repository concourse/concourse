package teamserver

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/api/accessor"
	"github.com/concourse/concourse/atc/api/present"
)

func (s *Server) GetTeam(w http.ResponseWriter, r *http.Request) {
	hLog := s.logger.Session("get-team")

	hLog.Debug("getting-team")

	teamName := r.FormValue(":team_name")
	team, found, err := s.teamFactory.FindTeam(teamName)
	if err != nil {
		hLog.Error("failed-to-get-team", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if !found {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	acc := accessor.GetAccessor(r)
	var presentedTeam atc.Team

	if acc.IsAdmin() || acc.IsAuthorized(team.Name()) {
		presentedTeam = present.Team(team)
	} else {
		hLog.Error("unauthorized", errors.New("not authorized to "+team.Name()))
		w.WriteHeader(http.StatusForbidden)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(presentedTeam); err != nil {
		hLog.Error("failed-to-encode-team", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	return
}
