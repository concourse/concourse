package wrappa

import (
	"net/http"

	"code.cloudfoundry.org/lager"

	"github.com/tedsuo/rata"
)

//go:generate counterfeiter code.cloudfoundry.org/lager.Logger

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
		defer release(wrappa.logger, pool)
		handler.ServeHTTP(w, r)
	})
}

func release(logger lager.Logger, pool Pool) {
	err := pool.Release()
	if err != nil {
		logger.Error("failed-to-release-handler-pool", err)
	}
}
