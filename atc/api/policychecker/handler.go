package policychecker

import (
	"fmt"
	"net/http"

	"code.cloudfoundry.org/lager/v3"

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

	logger := h.logger.Session("policy-checker")
	result, err := h.policyChecker.Check(h.action, acc, r)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		userMsg := "policy-checker: unreachable or misconfigured"
		// error should be logged and meaningful to the Concourse operator/maintainer
		logger.Error("policy-checker", err, lager.Data{"user-msg": userMsg})
		// actual error is not useful and can be confusing to Concourse user
		fmt.Fprint(w, userMsg)
		return
	}

	if !result.Allowed() {
		policyCheckErr := policy.PolicyCheckNotPass{
			Messages: result.Messages(),
		}
		if result.ShouldBlock() {
			w.WriteHeader(http.StatusForbidden)
			fmt.Fprint(w, policyCheckErr.Error())
			return
		} else {
			w.Header().Add("X-Concourse-Policy-Check-Warning", policyCheckErr.Error())
		}
	}

	h.handler.ServeHTTP(w, r)
}
