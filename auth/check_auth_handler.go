package auth

import "net/http"

type checkAuthHandler struct {
	handler  http.Handler
	rejector Rejector
}

func CheckAuthenticationHandler(
	handler http.Handler,
	rejector Rejector,
) http.Handler {
	return checkAuthHandler{
		handler:  handler,
		rejector: rejector,
	}
}

func (h checkAuthHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if IsAuthenticated(r) {
		h.handler.ServeHTTP(w, r)
	} else {
		h.rejector.Unauthorized(w, r)
	}
}
