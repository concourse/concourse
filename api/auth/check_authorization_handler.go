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
	if !IsAuthenticated(r) {
		h.rejector.Unauthorized(w, r)
		return
	}

	if !IsAuthorized(r) {
		h.rejector.Forbidden(w, r)
		return
	}

	h.handler.ServeHTTP(w, r)
}
