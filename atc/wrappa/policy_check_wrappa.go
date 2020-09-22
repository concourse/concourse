package wrappa

import (
	"code.cloudfoundry.org/lager"
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/api/policychecker"
	"github.com/tedsuo/rata"
)

func NewPolicyCheckWrappa(
	logger lager.Logger,
	checker policychecker.PolicyChecker,
) *PolicyCheckWrappa {
	return &PolicyCheckWrappa{logger, checker}
}

type PolicyCheckWrappa struct {
	logger  lager.Logger
	checker policychecker.PolicyChecker
}

func (w *PolicyCheckWrappa) Wrap(handlers rata.Handlers) rata.Handlers {
	wrapped := rata.Handlers{}

	for name, handler := range handlers {
		wrapped[name] = policychecker.NewHandler(w.logger, handler, atc.RouteAction(name), w.checker)
	}

	return wrapped
}
