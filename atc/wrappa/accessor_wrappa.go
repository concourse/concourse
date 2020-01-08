package wrappa

import (
	"code.cloudfoundry.org/lager"
	"github.com/concourse/concourse/atc/api/accessor"
	"github.com/concourse/concourse/atc/auditor"
	"github.com/tedsuo/rata"
)

func NewAccessorWrappa(logger lager.Logger, accessorFactory accessor.AccessFactory, aud auditor.Auditor) *AccessorWrappa {
	return &AccessorWrappa{logger, accessorFactory, aud}
}

type AccessorWrappa struct {
	logger          lager.Logger
	accessorFactory accessor.AccessFactory
	auditor         auditor.Auditor
}

func (w *AccessorWrappa) Wrap(handlers rata.Handlers) rata.Handlers {
	wrapped := rata.Handlers{}

	for name, handler := range handlers {
		wrapped[name] = accessor.NewHandler(w.logger, handler, w.accessorFactory, name, w.auditor)
	}

	return wrapped
}
