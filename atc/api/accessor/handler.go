package accessor

import (
	"context"
	"net/http"

	"code.cloudfoundry.org/lager"
)

func NewHandler(
	handler http.Handler,
	accessFactory AccessFactory,
	action string,
	logger lager.Logger,
) http.Handler {
	return accessorHandler{
		handler:       handler,
		accessFactory: accessFactory,
		action:        action,
		logger:        logger,
	}
}

type accessorHandler struct {
	handler       http.Handler
	accessFactory AccessFactory
	action        string
	logger        lager.Logger
}

func (h accessorHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	acc := h.accessFactory.Create(r, h.action)
	ctx := context.WithValue(r.Context(), "accessor", acc)

	if acc, ok := acc.(Access); ok {
		h.logger.Debug("audit", lager.Data{"action": r.URL.Path, "method": r.Method, "user": acc.UserName(), "parameters": r.URL.Query()})
	}
	h.handler.ServeHTTP(w, r.WithContext(ctx))
}

func GetAccessor(r *http.Request) Access {
	accessor := r.Context().Value("accessor")
	if accessor != nil {
		return accessor.(Access)
	}

	return &access{}
}
