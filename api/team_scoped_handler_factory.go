package api

import (
	"errors"
	"net/http"

	"code.cloudfoundry.org/lager"

	"github.com/concourse/atc/auth"
	"github.com/concourse/atc/db"
)

type TeamScopedHandlerFactory struct {
	logger        lager.Logger
	teamDBFactory db.TeamDBFactory
}

func NewTeamScopedHandlerFactory(
	logger lager.Logger,
	teamDBFactory db.TeamDBFactory,
) *TeamScopedHandlerFactory {
	return &TeamScopedHandlerFactory{
		logger:        logger,
		teamDBFactory: teamDBFactory,
	}
}

func (f *TeamScopedHandlerFactory) HandlerFor(teamScopedHandler func(db.TeamDB) http.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		logger := f.logger.Session("team-scoped-handler")
		teamName, _, _, found := auth.GetTeam(r)

		if !found {
			logger.Error("team-not-found-in-context", errors.New("team-not-found-in-context"))
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		teamDB := f.teamDBFactory.GetTeamDB(teamName)
		teamScopedHandler(teamDB).ServeHTTP(w, r)
	}
}
