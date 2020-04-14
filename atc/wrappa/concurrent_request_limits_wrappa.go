package wrappa

import (
	"net/http"

	"code.cloudfoundry.org/lager"

	"github.com/tedsuo/rata"
)

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

	for name, handler := range handlers {
		if wrappa.concurrentRequestPolicy.IsLimited(name) {
			wrapped[name] = wrapHandler(
				wrappa.logger,
				wrappa.concurrentRequestPolicy.MaxConcurrentRequests(name),
				handler,
			)
		} else {
			wrapped[name] = handler
		}
	}

	return wrapped
}

func wrapHandler(logger lager.Logger, limit int, handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	})
}
