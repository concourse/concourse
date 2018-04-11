package api

import (
	"errors"
	"net/http"

	"code.cloudfoundry.org/lager"

	"github.com/concourse/atc/api/accessor"
	"github.com/concourse/atc/db"
)

type TeamScopedHandlerFactory struct {
	logger      lager.Logger
	teamFactory db.TeamFactory
}

func NewTeamScopedHandlerFactory(
	logger lager.Logger,
	teamFactory db.TeamFactory,
) *TeamScopedHandlerFactory {
	return &TeamScopedHandlerFactory{
		logger:      logger,
		teamFactory: teamFactory,
	}
}

func (f *TeamScopedHandlerFactory) HandlerFor(teamScopedHandler func(db.Team) http.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		logger := f.logger.Session("team-scoped-handler")
		acc := accessor.GetAccessor(r)

		teamName := r.FormValue(":team_name")

		if acc.IsAuthorized(teamName) {
			team, found, err := f.teamFactory.FindTeam(teamName)
			if err != nil {
				logger.Error("failed-to-find-team-in-db", err)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}

			if !found {
				logger.Error("team-not-found-in-database", errors.New("team-not-found-in-database"), lager.Data{"team-name": teamName})
				w.WriteHeader(http.StatusNotFound)
				return
			}

			teamScopedHandler(team).ServeHTTP(w, r)
		} else {
			w.WriteHeader(http.StatusForbidden)
		}
	}
}
