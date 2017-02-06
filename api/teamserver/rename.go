package teamserver

import (
	"encoding/json"
	"errors"
	"io/ioutil"
	"net/http"

	"github.com/concourse/atc/api/present"
	"github.com/concourse/atc/auth"
	"github.com/concourse/atc/db"
)

func (s *Server) RenameTeam(w http.ResponseWriter, r *http.Request) {
	hLog := s.logger.Session("rename-team")
	hLog.Debug("renaming team")

	data, err := ioutil.ReadAll(r.Body)
	if err != nil {
		s.logger.Error("call-to-rename-team-copy-failed", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	var value struct{ Name string }
	err = json.Unmarshal(data, &value)
	if err != nil {
		s.logger.Error("call-to-rename-team-unmarshal-failed", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	authTeam, authTeamFound := auth.GetTeam(r)
	if !authTeamFound {
		hLog.Error("failed-to-get-team-from-auth", errors.New("failed-to-get-team-from-auth"))
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	teamName := r.FormValue(":team_name")
	if !authTeam.IsAdmin() && !authTeam.IsAuthorized(teamName) {
		w.WriteHeader(http.StatusForbidden)
		return
	}

	_, teamExists, err := s.getTeamByName(value.Name)
	if err != nil {
		s.logger.Error("sql-database-error", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if teamExists {
		w.WriteHeader(http.StatusConflict)
		return
	}

	savedTeam, found, err := s.getTeamByName(teamName)
	if err != nil {
		s.logger.Error("sql-database-error", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if found {
		hLog.Debug("updating team name")
		teamDb := s.teamDBFactory.GetTeamDB(teamName)
		savedTeam, err = teamDb.UpdateName(value.Name)

		if err != nil {
			hLog.Error("failed-to-rename-team", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
	} else {
		w.WriteHeader(http.StatusForbidden)
		return
	}

	json.NewEncoder(w).Encode(present.Team(savedTeam))
	return
}

func (s *Server) getTeamByName(teamName string) (db.SavedTeam, bool, error) {
	teamDB := s.teamDBFactory.GetTeamDB(teamName)
	return teamDB.GetTeam()
}
