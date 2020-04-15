package wrappa

import (
	"net/http"

	"code.cloudfoundry.org/lager"

	"github.com/tedsuo/rata"
)

//go:generate counterfeiter code.cloudfoundry.org/lager.Logger

type ConcurrencyLimitsWrappa struct {
	logger                  lager.Logger
	concurrentRequestPolicy ConcurrentRequestPolicy
}

func NewConcurrencyLimitsWrappa(
	logger lager.Logger,
	concurrentRequestPolicy ConcurrentRequestPolicy,
) Wrappa {
	return ConcurrencyLimitsWrappa{
		logger:                  logger,
		concurrentRequestPolicy: concurrentRequestPolicy,
	}
}

func (wrappa ConcurrencyLimitsWrappa) Wrap(handlers rata.Handlers) rata.Handlers {
	wrapped := rata.Handlers{}

	for action, handler := range handlers {
		if wrappa.concurrentRequestPolicy.IsLimited(action) {
			pool := wrappa.concurrentRequestPolicy.HandlerPool(action)
			wrapped[action] = wrappa.wrap(pool, handler)
		} else {
			wrapped[action] = handler
		}
	}

	return wrapped
}

func (wrappa ConcurrencyLimitsWrappa) wrap(
	pool Pool,
	handler http.Handler,
) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !pool.TryAcquire() {
			wrappa.logger.Info("concurrent-request-limit-reached")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		defer pool.Release()
		handler.ServeHTTP(w, r)
	})
}
