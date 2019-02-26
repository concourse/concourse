package wrappa

import (
	"github.com/concourse/concourse/atc/api/accessor"
	"github.com/concourse/concourse/atc/audit"
	"github.com/tedsuo/rata"
	"code.cloudfoundry.org/lager"
)

func NewAccessorWrappa(accessorFactory accessor.AccessFactory, aud audit.Audit) *AccessorWrappa {
	return &AccessorWrappa{accessorFactory, aud }

}

type AccessorWrappa struct {
	accessorFactory accessor.AccessFactory
	audit audit.Audit
}

func (w *AccessorWrappa) Wrap(handlers rata.Handlers) rata.Handlers {
	wrapped := rata.Handlers{}

	for name, handler := range handlers {
		wrapped[name] = accessor.NewHandler(handler, w.accessorFactory, name, w.audit)
	}

	return wrapped
}
