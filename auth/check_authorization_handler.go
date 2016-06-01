package auth

import (
	"net/http"

	"github.com/concourse/atc"
	"github.com/gorilla/context"
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
	if IsAuthenticated(r) {
		authTeamName, ok := context.GetOk(r, teamNameKey)
		if !ok {
			authTeamName = atc.DefaultTeamName
		}

		if r.URL.Query().Get(":team_name") == authTeamName {
			h.handler.ServeHTTP(w, r)
			return
		}
	}

	h.rejector.Unauthorized(w, r)
}
