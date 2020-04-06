package auth

import (
	"net/http"

	"code.cloudfoundry.org/lager"

	"github.com/concourse/concourse/skymarshal/token"
)

func CSRFValidationHandler(
	handler http.Handler,
	middleware token.Middleware,
) http.Handler {
	return csrfValidationHandler{
		handler:    handler,
		middleware: middleware,
	}
}

type csrfValidationHandler struct {
	handler    http.Handler
	middleware token.Middleware
}

func (h csrfValidationHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	logger, ok := r.Context().Value("logger").(lager.Logger)
	if !ok {
		panic("logger is not set in request context for csrf validation handler")
	}

	logger = logger.Session("csrf-validation")

	if IsCSRFRequired(r) {

		csrfHeader := r.Header.Get(CSRFHeaderName)
		if csrfHeader == "" {
			logger.Debug("csrf-header-is-not-set")
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		csrfToken := h.middleware.GetCSRFToken(r)
		if csrfToken == "" {
			logger.Debug("csrf-is-not-provided-in-auth-token")
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		if csrfToken != csrfHeader {
			logger.Debug("csrf-token-does-not-match-auth-token", lager.Data{
				"auth-csrf-token":    csrfToken,
				"request-csrf-token": csrfHeader,
			})
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
	}

	h.handler.ServeHTTP(w, r)
}
