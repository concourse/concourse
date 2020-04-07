package wrappa

import (
	"code.cloudfoundry.org/lager"
	"github.com/concourse/concourse/atc/api/accessor"
	"github.com/concourse/concourse/atc/auditor"
	"github.com/concourse/concourse/atc/db"
	"github.com/tedsuo/rata"
)

func NewAccessorWrappa(
	logger lager.Logger,
	accessorFactory accessor.AccessFactory,
	aud auditor.Auditor,
	userFactory db.UserFactory,
) *AccessorWrappa {
	return &AccessorWrappa{logger, accessorFactory, aud, userFactory}
}

type AccessorWrappa struct {
	logger          lager.Logger
	accessorFactory accessor.AccessFactory
	auditor         auditor.Auditor
	userFactory     db.UserFactory
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
			w.userFactory,
		)
	}

	return wrapped
}
