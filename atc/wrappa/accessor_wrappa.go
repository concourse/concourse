package wrappa

import (
	"code.cloudfoundry.org/lager"
	"github.com/concourse/concourse/atc/api/accessor"
	"github.com/concourse/concourse/atc/auditor"
	"github.com/tedsuo/rata"
)

func NewAccessorWrappa(
	logger lager.Logger,
	accessorFactory accessor.AccessFactory,
	aud auditor.Auditor,
	userTracker accessor.UserTracker,
) *AccessorWrappa {
	return &AccessorWrappa{logger, accessorFactory, aud, userTracker}
}

type AccessorWrappa struct {
	logger          lager.Logger
	accessorFactory accessor.AccessFactory
	auditor         auditor.Auditor
	userTracker     accessor.UserTracker
}

func (w *AccessorWrappa) Wrap(handlers rata.Handlers) rata.Handlers {
	wrapped := rata.Handlers{}

	for name, handler := range handlers {
		wrapped[name] = accessor.NewHandler(
			w.logger,
			handler,
			w.accessorFactory,
			name,
			w.auditor,
			w.userTracker,
		)
	}

	return wrapped
}
