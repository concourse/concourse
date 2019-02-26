package wrappa

import (
	"github.com/concourse/concourse/atc/api/accessor"
	"github.com/tedsuo/rata"
	"code.cloudfoundry.org/lager"
)

func NewAccessorWrappa(accessorFactory accessor.AccessFactory, logger lager.Logger) *AccessorWrappa {
	return &AccessorWrappa{accessorFactory, logger}
}

type AccessorWrappa struct {
	accessorFactory accessor.AccessFactory
	logger lager.Logger
}

func (w *AccessorWrappa) Wrap(handlers rata.Handlers) rata.Handlers {
	wrapped := rata.Handlers{}

	for name, handler := range handlers {
		wrapped[name] = accessor.NewHandler(handler, w.accessorFactory, name, w.logger)
	}

	return wrapped
}
