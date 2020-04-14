package wrappa

import (
	"code.cloudfoundry.org/lager"
	"github.com/concourse/concourse/atc/api/accessor"
	"github.com/concourse/concourse/atc/auditor"
	"github.com/tedsuo/rata"
)

func NewAccessorWrappa(
	logger lager.Logger,
	accessFactory accessor.AccessFactory,
	tokenVerifier accessor.TokenVerifier,
	teamFetcher accessor.TeamFetcher,
	userTracker accessor.UserTracker,
	auditor auditor.Auditor,
	customRoles map[string]string,
) *AccessorWrappa {
	return &AccessorWrappa{
		logger:        logger,
		accessFactory: accessFactory,
		tokenVerifier: tokenVerifier,
		teamFetcher:   teamFetcher,
		userTracker:   userTracker,
		auditor:       auditor,
		customRoles:   customRoles,
	}
}

type AccessorWrappa struct {
	logger        lager.Logger
	accessFactory accessor.AccessFactory
	tokenVerifier accessor.TokenVerifier
	teamFetcher   accessor.TeamFetcher
	userTracker   accessor.UserTracker
	auditor       auditor.Auditor
	customRoles   map[string]string
}

func (w *AccessorWrappa) Wrap(handlers rata.Handlers) rata.Handlers {
	wrapped := rata.Handlers{}

	for name, handler := range handlers {
		wrapped[name] = accessor.NewHandler(
			w.logger,
			name,
			handler,
			w.accessFactory,
			w.tokenVerifier,
			w.teamFetcher,
			w.userTracker,
			w.auditor,
			w.customRoles,
		)
	}

	return wrapped
}
