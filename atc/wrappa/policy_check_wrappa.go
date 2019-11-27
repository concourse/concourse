package wrappa

import (
	"code.cloudfoundry.org/lager"
	"github.com/concourse/concourse/atc/api/policychecker"
	"github.com/concourse/concourse/atc/policy"
	"github.com/tedsuo/rata"
)

func NewPolicyCheckWrappa(
	logger lager.Logger,
	checker policy.PreChecker,
) *PolicyCheckWrappa {
	return &PolicyCheckWrappa{logger, checker}
}

type PolicyCheckWrappa struct {
	logger  lager.Logger
	checker policy.PreChecker
}

func (w *PolicyCheckWrappa) Wrap(handlers rata.Handlers) rata.Handlers {
	wrapped := rata.Handlers{}

	for name, handler := range handlers {
		wrapped[name] = policychecker.NewHandler(w.logger, handler, name, w.checker)
	}

	return wrapped
}
