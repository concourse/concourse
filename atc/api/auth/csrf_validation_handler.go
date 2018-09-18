package auth

import (
	"net/http"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/atc/api/accessor"
)

func CSRFValidationHandler(
	handler http.Handler,
	rejector Rejector,
) http.Handler {
	return csrfValidationHandler{
		handler:  handler,
		rejector: rejector,
	}
}

type csrfValidationHandler struct {
	handler  http.Handler
	rejector Rejector
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
			h.rejector.Unauthorized(w, r)
			return
		}

		acc := accessor.GetAccessor(r)
		csrfToken := acc.CSRFToken()
		if csrfToken == "" {
			logger.Debug("csrf-is-not-provided-in-auth-token")
			h.rejector.Unauthorized(w, r)
			return
		}

		if csrfToken != csrfHeader {
			logger.Debug("csrf-token-does-not-match-auth-token", lager.Data{
				"auth-csrf-token":    csrfToken,
				"request-csrf-token": csrfHeader,
			})
			h.rejector.Unauthorized(w, r)
			return
		}
	}

	h.handler.ServeHTTP(w, r)
}
