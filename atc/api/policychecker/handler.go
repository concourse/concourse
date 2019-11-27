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
	policyChecker policy.PreChecker,
) http.Handler {
	return policyCheckerHandler{
		handler:       handler,
		action:        action,
		policyChecker: policyChecker,
	}
}

type policyCheckerHandler struct {
	logger        lager.Logger
	handler       http.Handler
	action        string
	policyChecker policy.PreChecker
}

func (h policyCheckerHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	acc := accessor.GetAccessor(r)
	ctx := context.WithValue(r.Context(), "accessor", acc)

	pass, err := h.policyChecker.CheckHttpApi(h.action, acc.UserName(), r)
	if err != nil {
		rejector := NotPassRejector{msg: err.Error()}
		rejector.Fail(w, r)
		return
	}
	if !pass {
		rejector := NotPassRejector{}
		rejector.NotPass(w, r)
		return
	}

	h.handler.ServeHTTP(w, r.WithContext(ctx))
}
