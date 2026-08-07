package auth

import (
	"net/http"

	"github.com/concourse/concourse/atc/api/accessor"
)

type checkSystemAccessHandler struct {
	handler  http.Handler
	rejector Rejector
}

func CheckSystemAccessHandler(
	handler http.Handler,
	rejector Rejector,
) http.Handler {
	return checkSystemAccessHandler{
		handler:  handler,
		rejector: rejector,
	}
}

func (h checkSystemAccessHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	acc := accessor.GetAccessor(r)

	if !acc.IsAuthenticated() {
		h.rejector.Unauthorized(w, r)
		return
	}

	if !acc.IsSystem() {
		h.rejector.Forbidden(w, r)
		return
	}

	h.handler.ServeHTTP(w, r)
}
