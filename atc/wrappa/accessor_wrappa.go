package wrappa

import (
	"net/http"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/concourse/atc/api/accessor"
	"github.com/concourse/concourse/atc/auditor"
)

func NewAccessorWrappa(
	logger lager.Logger,
	accessFactory accessor.AccessFactory,
	auditor auditor.Auditor,
	customRoles map[string]string,
) *AccessorWrappa {
	return &AccessorWrappa{
		logger:        logger,
		accessFactory: accessFactory,
		auditor:       auditor,
		customRoles:   customRoles,
	}
}

type AccessorWrappa struct {
	logger        lager.Logger
	accessFactory accessor.AccessFactory
	auditor       auditor.Auditor
	customRoles   map[string]string
}

func (w *AccessorWrappa) Wrap(handlers map[string]http.Handler) map[string]http.Handler {
	wrapped := map[string]http.Handler{}

	for name, handler := range handlers {
		wrapped[name] = accessor.NewHandler(
			w.logger,
			name,
			handler,
			w.accessFactory,
			w.auditor,
			w.customRoles,
		)
	}

	return wrapped
}
