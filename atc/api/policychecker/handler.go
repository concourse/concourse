package policychecker

import (
	"code.cloudfoundry.org/lager"
	"context"
	"net/http"

	"github.com/concourse/concourse/atc/api/accessor"
	"github.com/concourse/concourse/atc/policy"
)

func NewHandler(
	logger lager.Logger,
	handler http.Handler,
	action string,
	policyChecker policy.Checker,
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
	policyChecker policy.Checker
}

func (h policyCheckingHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	acc := accessor.GetAccessor(r)
	ctx := context.WithValue(r.Context(), "accessor", acc)

	if h.policyChecker != nil {
		pass, err := h.policyChecker.CheckHttpApi(h.action, acc, r)
		if err != nil {
			rejector := NotPassRejector{msg: err.Error()}
			rejector.Error(w, r)
			return
		}
		if !pass {
			rejector := NotPassRejector{}
			rejector.Reject(w, r)
			return
		}
	}

	h.handler.ServeHTTP(w, r.WithContext(ctx))
}
