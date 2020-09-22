package wrappa

import (
	"net/http"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/metric"
	"github.com/tedsuo/rata"
)

type ConcurrentRequestLimitsWrappa struct {
	logger                  lager.Logger
	concurrentRequestPolicy ConcurrentRequestPolicy
}

func NewConcurrentRequestLimitsWrappa(
	logger lager.Logger,
	concurrentRequestPolicy ConcurrentRequestPolicy,
) Wrappa {
	return ConcurrentRequestLimitsWrappa{
		logger:                  logger,
		concurrentRequestPolicy: concurrentRequestPolicy,
	}
}

func (wrappa ConcurrentRequestLimitsWrappa) Wrap(
	handlers rata.Handlers,
) rata.Handlers {
	wrapped := rata.Handlers{}

	for name, handler := range handlers {
		action := atc.RouteAction(name)
		pool, found := wrappa.concurrentRequestPolicy.HandlerPool(action)
		if found {
			inflight := &metric.Gauge{}
			limitHit := &metric.Counter{}

			metric.Metrics.ConcurrentRequests[action] = inflight
			metric.Metrics.ConcurrentRequestsLimitHit[action] = limitHit

			wrapped[name] = wrappa.wrap(
				pool,
				handler,
				inflight, limitHit,
			)
		} else {
			wrapped[name] = handler
		}
	}

	return wrapped
}

func (wrappa ConcurrentRequestLimitsWrappa) wrap(
	pool Pool,
	handler http.Handler,
	inflight *metric.Gauge, limitHit *metric.Counter,
) http.Handler {
	if pool.Size() == 0 {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			wrappa.logger.Debug("endpoint-disabled")
			limitHit.Inc()

			w.WriteHeader(http.StatusNotImplemented)
		})
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !pool.TryAcquire() {
			wrappa.logger.Info("concurrent-request-limit-reached")
			limitHit.Inc()

			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}

		defer inflight.Dec()
		defer pool.Release()

		inflight.Inc()
		handler.ServeHTTP(w, r)
	})
}
