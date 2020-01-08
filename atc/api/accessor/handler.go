package accessor

import (
	"context"
	"net/http"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/concourse/atc/auditor"
)

func NewHandler(
	logger lager.Logger,
	handler http.Handler,
	accessFactory AccessFactory,
	action string,
	aud auditor.Auditor,
) http.Handler {
	return accessorHandler{
		logger:        logger,
		handler:       handler,
		accessFactory: accessFactory,
		action:        action,
		auditor:       aud,
	}
}

type accessorHandler struct {
	logger        lager.Logger
	handler       http.Handler
	accessFactory AccessFactory
	action        string
	auditor       auditor.Auditor
}

func (h accessorHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {

	acc, err := h.accessFactory.Create(r, h.action)
	if err != nil {
		h.logger.Error("failed-to-create-accessor", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	ctx := context.WithValue(r.Context(), "accessor", acc)

	h.auditor.Audit(h.action, acc.UserName(), r)
	h.handler.ServeHTTP(w, r.WithContext(ctx))
}

func GetAccessor(r *http.Request) Access {
	accessor := r.Context().Value("accessor")
	if accessor != nil {
		return accessor.(Access)
	}

	return &access{}
}
