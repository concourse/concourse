package auth

import (
	"net/http"

	"github.com/concourse/concourse/atc/api/accessor"
)

type checkAnyTeamAccessHandler struct {
	handler  http.Handler
	rejector Rejector
}

func CheckAnyTeamAccessHandler(
	handler http.Handler,
	rejector Rejector,
) http.Handler {
	return checkAnyTeamAccessHandler{
		handler:  handler,
		rejector: rejector,
	}
}

func (h checkAnyTeamAccessHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	acc := accessor.GetAccessor(r)

	if !acc.IsAuthenticated() {
		h.rejector.Unauthorized(w, r)
		return
	}

	if !acc.IsAdmin() && len(acc.TeamNames()) == 0 {
		h.rejector.Forbidden(w, r)
		return
	}

	h.handler.ServeHTTP(w, r)
}
