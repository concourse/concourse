package accessor

import (
	"context"
	"net/http"
)

func NewHandler(
	handler http.Handler,
	accessFactory AccessFactory,
	action string,
) http.Handler {
	return accessorHandler{
		handler:       handler,
		accessFactory: accessFactory,
		action:        action,
	}
}

type accessorHandler struct {
	handler       http.Handler
	accessFactory AccessFactory
	action        string
}

func (h accessorHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {

	acc := h.accessFactory.Create(r, h.action)
	ctx := context.WithValue(r.Context(), "accessor", acc)

	h.handler.ServeHTTP(w, r.WithContext(ctx))
}

func GetAccessor(r *http.Request) Access {
	accessor := r.Context().Value("accessor")
	if accessor != nil {
		return accessor.(Access)
	}

	return &access{}
}
