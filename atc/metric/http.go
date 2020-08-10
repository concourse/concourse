package metric

import (
	"net/http"

	"code.cloudfoundry.org/lager"
	"github.com/felixge/httpsnoop"
)

type MetricsHandler struct {
	Logger  lager.Logger
	Route   string
	Handler http.Handler
	Monitor *Monitor
}

func WrapHandler(
	logger lager.Logger,
	monitor *Monitor,
	route string,
	handler http.Handler,
) http.Handler {
	return MetricsHandler{
		Logger:  logger,
		Monitor: monitor,
		Route:   route,
		Handler: handler,
	}
}

func (handler MetricsHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	metrics := httpsnoop.CaptureMetrics(handler.Handler, w, r)

	HTTPResponseTime{
		Route:      handler.Route,
		Path:       r.URL.Path,
		Method:     r.Method,
		StatusCode: metrics.Code,
		Duration:   metrics.Duration,
	}.Emit(handler.Logger, handler.Monitor)
}
