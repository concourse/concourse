package accessor

import (
	"context"
	"net/http"
	"reflect"
	"github.com/concourse/concourse/atc/audit"


	"github.com/concourse/concourse/atc/audit"
>>>>>>> creation of audit package
)

func NewHandler(
	handler http.Handler,
	accessFactory AccessFactory,
	action string,
	aud audit.Audit,
) http.Handler {
	return accessorHandler{
		handler:       handler,
		accessFactory: accessFactory,
		action:        action,
		audit:         aud,
	}
}

type accessorHandler struct {
	handler       http.Handler
	accessFactory AccessFactory
	action        string
	audit         audit.Audit
}

func (h accessorHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	acc := h.accessFactory.Create(r, h.action)
	ctx := context.WithValue(r.Context(), "accessor", acc)

	if reflect.TypeOf(acc) == reflect.TypeOf(&access{}) && acc != nil {
		h.audit.LogAction(h.action, acc.UserName(), r)
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
