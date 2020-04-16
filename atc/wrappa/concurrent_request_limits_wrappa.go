package wrappa

import (
	"net/http"

	"code.cloudfoundry.org/lager"

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

func (wrappa ConcurrentRequestLimitsWrappa) Wrap(handlers rata.Handlers) rata.Handlers {
	wrapped := rata.Handlers{}

	for action, handler := range handlers {
		pool, found := wrappa.concurrentRequestPolicy.HandlerPool(action)
		if found {
			wrapped[action] = wrappa.wrap(pool, handler)
		} else {
			wrapped[action] = handler
		}
	}

	return wrapped
}

func (wrappa ConcurrentRequestLimitsWrappa) wrap(
	pool Pool,
	handler http.Handler,
) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !pool.TryAcquire() {
			wrappa.logger.Info("concurrent-request-limit-reached")
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		defer pool.Release()
		handler.ServeHTTP(w, r)
	})
}
