package wrappa

import (
	"github.com/concourse/concourse/atc/api/accessor"
	"github.com/tedsuo/rata"
)

func NewAccessorWrappa(accessorFactory accessor.AccessFactory) *AccessorWrappa {
	return &AccessorWrappa{accessorFactory}
}

type AccessorWrappa struct {
	accessorFactory accessor.AccessFactory
}

func (w *AccessorWrappa) Wrap(handlers rata.Handlers) rata.Handlers {
	wrapped := rata.Handlers{}

	for name, handler := range handlers {
		wrapped[name] = accessor.NewHandler(handler, w.accessorFactory, name)
	}

	return wrapped
}
