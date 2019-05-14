package wrappa

import (
	"github.com/concourse/concourse/atc/api/accessor"
	"github.com/concourse/concourse/atc/auditor"
	"github.com/tedsuo/rata"
)

func NewAccessorWrappa(accessorFactory accessor.AccessFactory, aud auditor.Auditor) *AccessorWrappa {
	return &AccessorWrappa{accessorFactory, aud}
}

type AccessorWrappa struct {
	accessorFactory accessor.AccessFactory
	auditor         auditor.Auditor
}

func (w *AccessorWrappa) Wrap(handlers rata.Handlers) rata.Handlers {
	wrapped := rata.Handlers{}

	for name, handler := range handlers {
		wrapped[name] = accessor.NewHandler(handler, w.accessorFactory, name, w.auditor)
	}

	return wrapped
}
