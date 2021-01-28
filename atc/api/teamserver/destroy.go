package teamserver

import (
	"net/http"

	"github.com/concourse/concourse/atc/db"
)

func (s *Server) DestroyTeam(team db.Team) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hLog := s.logger.Session("destroy-team")
		hLog.Debug("destroying-team")

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

		err := team.Delete()
		if err != nil {
			hLog.Error("failed-to-delete-team", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusNoContent)
	})
}
