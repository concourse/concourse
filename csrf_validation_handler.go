package auth

import "net/http"

const CSRFHeaderName = "X-CSRF-Token"

func CSRFValidationHandler(
	handler http.Handler,
	rejector Rejector,
	userContextReader UserContextReader,
) http.Handler {
	return csrfValidationHandler{
		handler:           handler,
		rejector:          rejector,
		userContextReader: userContextReader,
	}
}

type csrfValidationHandler struct {
	handler           http.Handler
	rejector          Rejector
	userContextReader UserContextReader
}

func (h csrfValidationHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// logger, ok := r.Context().Value("logger").(lager.Logger)
	// if !ok {
	// 	panic("logger is not set in request context for csrf validation handler")
	// }
	//
	// logger = logger.Session("csrf-validation")
	//
	// isCSRFRequired, ok := r.Context().Value(CSRFRequiredKey).(bool)
	// if ok && isCSRFRequired {
	// 	if r.Header.Get(CSRFHeaderName) == "" {
	// 		logger.Debug("csrf-header-is-not-set")
	// 		w.WriteHeader(http.StatusBadRequest)
	// 		return
	// 	}
	//
	// 	authCSRFToken, authCSRFTokenProvided := h.userContextReader.GetCSRFToken(r)
	// 	if !authCSRFTokenProvided {
	// 		logger.Debug("csrf-is-not-provided-in-auth-token")
	// 		w.WriteHeader(http.StatusBadRequest)
	// 		return
	// 	}
	//
	// 	if authCSRFToken != r.Header.Get(CSRFHeaderName) {
	// 		logger.Debug("csrf-token-does-not-match-auth-token", lager.Data{
	// 			"auth-csrf-token":    authCSRFToken,
	// 			"request-csrf-token": r.Header.Get(CSRFHeaderName),
	// 		})
	// 		h.rejector.Unauthorized(w, r)
	// 		return
	// 	}
	// }

	h.handler.ServeHTTP(w, r)
}
