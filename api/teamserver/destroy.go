package teamserver

import (
	"errors"
	"net/http"

	"github.com/concourse/atc/api/auth"
)

func (s *Server) DestroyTeam(w http.ResponseWriter, r *http.Request) {
	hLog := s.logger.Session("destroy-team")
	hLog.Debug("destroying-team")

	authTeam, authTeamFound := auth.GetTeam(r)
	if !authTeamFound {
		hLog.Error("failed-to-get-team-from-auth", errors.New("failed-to-get-team-from-auth"))
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	teamName := r.FormValue(":team_name")

	if !authTeam.IsAdmin() {
		hLog.Info("requesting-team-is-not-admin")
		w.WriteHeader(http.StatusForbidden)
		return
	}

	team, found, err := s.teamFactory.FindTeam(teamName)
	if err != nil {
		hLog.Error("failed-to-get-team", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if !found {
		hLog.Info("team-not-found")
		w.WriteHeader(http.StatusNotFound)
		return
	}

	if team.Admin() {
		allTeams, err := s.teamFactory.GetTeams()
		if err != nil {
			hLog.Error("failed-to-get-teams", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		adminTeams := 0
		for _, candidate := range allTeams {
			if candidate.Admin() {
				adminTeams = adminTeams + 1
			}
		}

		if adminTeams == 1 {
			hLog.Info("team-is-last-admin-team")
			w.WriteHeader(http.StatusForbidden)
			return
		}
	}

	err = team.Delete()
	if err != nil {
		hLog.Error("failed-to-delete-team", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
