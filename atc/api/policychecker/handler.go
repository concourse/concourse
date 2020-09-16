package policychecker

import (
	"fmt"
	"net/http"

	"code.cloudfoundry.org/lager"

	"github.com/concourse/concourse/atc/api/accessor"
	"github.com/concourse/concourse/atc/policy"
)

func NewHandler(
	logger lager.Logger,
	handler http.Handler,
	action string,
	policyChecker PolicyChecker,
) http.Handler {
	return policyCheckingHandler{
		logger:        logger,
		handler:       handler,
		action:        action,
		policyChecker: policyChecker,
	}
}

type policyCheckingHandler struct {
	logger        lager.Logger
	handler       http.Handler
	action        string
	policyChecker PolicyChecker
}

func (h policyCheckingHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	acc := accessor.GetAccessor(r)

	if h.policyChecker != nil {
		result, err := h.policyChecker.Check(h.action, acc, r)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprintf(w, fmt.Sprintf("policy check error: %s", err.Error()))
			return
		}
		if !result.Allowed {
			w.WriteHeader(http.StatusForbidden)
			policyCheckErr := policy.PolicyCheckNotPass{
				Reasons: result.Reasons,
			}
			fmt.Fprintf(w, policyCheckErr.Error())
			return
		}
	}

	h.handler.ServeHTTP(w, r)
}
