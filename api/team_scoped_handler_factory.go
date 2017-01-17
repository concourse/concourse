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
	logger           lager.Logger
	teamDBFactory    db.TeamDBFactory
	containerFactory dbng.ContainerFactory
}

func NewTeamScopedHandlerFactory(
	logger lager.Logger,
	teamDBFactory db.TeamDBFactory,
	containerFactory dbng.ContainerFactory,
) *TeamScopedHandlerFactory {
	return &TeamScopedHandlerFactory{
		logger:           logger,
		teamDBFactory:    teamDBFactory,
		containerFactory: containerFactory,
	}
}

func (f *TeamScopedHandlerFactory) HandlerFor(teamScopedHandler func(db.TeamDB, dbng.ContainerFactory) http.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		logger := f.logger.Session("team-scoped-handler")

		authTeam, authTeamFound := auth.GetTeam(r)
		if !authTeamFound {
			logger.Error("team-not-found-in-context", errors.New("team-not-found-in-context"))
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		teamDB := f.teamDBFactory.GetTeamDB(authTeam.Name())
		teamScopedHandler(teamDB, f.containerFactory).ServeHTTP(w, r)
	}
}
