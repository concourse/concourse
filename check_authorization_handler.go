package auth

import "net/http"

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
	authorized, response := IsAuthorized(r)
	if authorized {
		h.handler.ServeHTTP(w, r)
		return
	}

	if response == Unauthorized {
		h.rejector.Unauthorized(w, r)
	} else if response == Forbidden {
		h.rejector.Forbidden(w, r)
	}
}
