package accessor

import (
	"context"
	"net/http"

	"github.com/concourse/concourse/atc/auditor"
)

func NewHandler(
	handler http.Handler,
	accessFactory AccessFactory,
	action string,
	aud auditor.Auditor,
) http.Handler {
	return accessorHandler{
		handler:       handler,
		accessFactory: accessFactory,
		action:        action,
		auditor:         aud,
	}
}

type accessorHandler struct {
	handler       http.Handler
	accessFactory AccessFactory
	action        string
	auditor         auditor.Auditor
}

func (h accessorHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	acc := h.accessFactory.Create(r, h.action)
	ctx := context.WithValue(r.Context(), "accessor", acc)

	h.auditor.LogAction(h.action, acc.UserName(), r)
	h.handler.ServeHTTP(w, r.WithContext(ctx))
}

func GetAccessor(r *http.Request) Access {
	accessor := r.Context().Value("accessor")
	if accessor != nil {
		return accessor.(Access)
	}

	return &access{}
}
