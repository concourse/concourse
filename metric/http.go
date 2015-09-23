package metric

import (
	"net/http"
	"time"

	"github.com/pivotal-golang/lager"
)

func WrapHandler(route string, handler http.Handler, logger lager.Logger) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		handler.ServeHTTP(w, r)

		HTTPReponseTime{
			Route:    route,
			Path:     r.URL.Path,
			Duration: time.Since(start),
		}.Emit(logger)
	})
}
