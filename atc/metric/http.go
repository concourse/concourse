package metric

import (
	"net/http"
	"time"

	"code.cloudfoundry.org/lager"
)

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
	handler.Handler.ServeHTTP(w, r)

	HTTPResponseTime{
		Route:    handler.Route,
		Path:     r.URL.Path,
		Method:   r.Method,
		Duration: time.Since(start),
	}.Emit(handler.Logger)
}
