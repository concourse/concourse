package metric

import (
	"net/http"
	"time"

	"code.cloudfoundry.org/lager"
)

type statusCodeResponseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (s *statusCodeResponseWriter) WriteHeader(code int) {
	s.statusCode = code
	s.ResponseWriter.WriteHeader(code)
}

type MetricsHandler struct {
	Logger lager.Logger

	Route   string
	Handler http.Handler
}

func WrapHandler(logger lager.Logger, route string, handler http.Handler) http.Handler {
	return MetricsHandler{
		Logger:  logger,
		Route:   route,
		Handler: handler,
	}
}

func (handler MetricsHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	start := time.Now()

	statusResponseWriter := &statusCodeResponseWriter{w, http.StatusOK}
	handler.Handler.ServeHTTP(statusResponseWriter, r)

	HTTPResponseTime{
		Route:      handler.Route,
		Path:       r.URL.Path,
		Method:     r.Method,
		StatusCode: statusResponseWriter.statusCode,
		Duration:   time.Since(start),
	}.Emit(handler.Logger)
}
