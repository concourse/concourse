package api

import (
	"errors"
	"net/http"

	"code.cloudfoundry.org/lager"

	"github.com/concourse/atc/auth"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/dbng"
)

type TeamScopedHandlerFactory struct {
	logger        lager.Logger
	teamDBFactory db.TeamDBFactory
	teamFactory   dbng.TeamFactory
}

func NewTeamScopedHandlerFactory(
	logger lager.Logger,
	teamDBFactory db.TeamDBFactory,
	teamFactory dbng.TeamFactory,
) *TeamScopedHandlerFactory {
	return &TeamScopedHandlerFactory{
		logger:        logger,
		teamDBFactory: teamDBFactory,
		teamFactory:   teamFactory,
	}
}

func (f *TeamScopedHandlerFactory) HandlerFor(teamScopedHandler func(db.TeamDB, dbng.Team) http.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		logger := f.logger.Session("team-scoped-handler")

		authTeam, authTeamFound := auth.GetTeam(r)
		if !authTeamFound {
			logger.Error("team-not-found-in-context", errors.New("team-not-found-in-context"))
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		teamDB := f.teamDBFactory.GetTeamDB(authTeam.Name())
		team, found, err := f.teamFactory.FindTeam(authTeam.Name())
		if err != nil {
			logger.Error("failed-to-find-team-in-db", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		if !found {
			logger.Error("team-not-found-in-database", errors.New("team-not-found-in-database"), lager.Data{"team-name": authTeam.Name()})
			w.WriteHeader(http.StatusNotFound)
			return
		}

		teamScopedHandler(teamDB, team).ServeHTTP(w, r)
	}
}
