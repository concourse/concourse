package wrappa

import (
	"net/http"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/concourse/atc/api/policychecker"
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

func (w *PolicyCheckWrappa) Wrap(handlers map[string]http.Handler) map[string]http.Handler {
	wrapped := map[string]http.Handler{}

	for name, handler := range handlers {
		wrapped[name] = policychecker.NewHandler(w.logger, handler, name, w.checker)
	}

	return wrapped
}
