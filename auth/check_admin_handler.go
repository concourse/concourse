package auth

import "net/http"

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
	if IsAuthenticated(r) {
		if IsAdmin(r) {
			h.handler.ServeHTTP(w, r)
		} else {
			h.rejector.Forbidden(w, r)
		}
	} else {
		h.rejector.Unauthorized(w, r)
	}
}
