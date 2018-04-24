package auth

import (
	"net/http"

	"github.com/concourse/atc/api/accessor"
)

type checkAuthorizationHandler struct {
	handler  http.Handler
	rejector Rejector
}

func CheckAuthorizationHandler(
	handler http.Handler,
	rejector Rejector,
) http.Handler {
	return checkAuthorizationHandler{
		handler:  handler,
		rejector: rejector,
	}
}

func (h checkAuthorizationHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	acc := accessor.GetAccessor(r)

	if !acc.IsAuthenticated() {
		h.rejector.Unauthorized(w, r)
		return
	}

	teamName := r.URL.Query().Get(":team_name")

	if !acc.IsAuthorized(teamName) {
		h.rejector.Forbidden(w, r)
		return
	}

	h.handler.ServeHTTP(w, r)
}
