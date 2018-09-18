package auth

import (
	"net/http"

	"github.com/concourse/atc/api/accessor"
)

type checkAdminHandler struct {
	handler  http.Handler
	rejector Rejector
}

func CheckAdminHandler(
	handler http.Handler,
	rejector Rejector,
) http.Handler {
	return checkAdminHandler{
		handler:  handler,
		rejector: rejector,
	}
}

func (h checkAdminHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	acc := accessor.GetAccessor(r)
	if acc.IsAuthenticated() {
		if acc.IsAdmin() {
			h.handler.ServeHTTP(w, r)
		} else {
			h.rejector.Forbidden(w, r)
		}
	} else {
		h.rejector.Unauthorized(w, r)
	}
}
