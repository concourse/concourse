package metrics

import (
	"net/http"
	"strconv"

	"github.com/felixge/httpsnoop"
	"github.com/prometheus/client_golang/prometheus"
)

type MetricsHandler struct {
	Route   string
	Handler http.Handler
}

func WrapHandler(handler http.Handler) http.Handler {
	return MetricsHandler{
		Handler: handler,
	}
}

func (handler MetricsHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	metrics := httpsnoop.CaptureMetrics(handler.Handler, w, r)

	HttpResponseDuration.
		With(prometheus.Labels{
			"code":  strconv.Itoa(metrics.Code),
			"route": handler.Route,
		}).
		Observe(metrics.Duration.Seconds())
}
