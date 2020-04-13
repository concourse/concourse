package wrappa

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"code.cloudfoundry.org/lager"

	// "github.com/concourse/concourse/atc/metric"
	"github.com/tedsuo/rata"
)

type ConcurrentRequestLimitFlag struct {
	Action string
	Limit  int
}

func (crl *ConcurrentRequestLimitFlag) UnmarshalFlag(value string) error {
	vs := strings.SplitN(value, "=", 2)
	if len(vs) != 2 {
		return fmt.Errorf("invalid concurrent request limit '%s' (must be <api action>=<non-negative integer>)", value)
	}

	crl.Action = vs[0]
	limit, err := strconv.Atoi(vs[1])
	if err != nil {
		return err
	}
	crl.Limit = limit

	return nil
}

type ConcurrencyLimitsWrappa struct {
	logger                  lager.Logger
	concurrentRequestLimits []ConcurrentRequestLimitFlag
}

func NewConcurrencyLimitsWrappa(
	logger lager.Logger,
	concurrentRequestLimits []ConcurrentRequestLimitFlag,
) Wrappa {
	return ConcurrencyLimitsWrappa{
		logger:                  logger,
		concurrentRequestLimits: concurrentRequestLimits,
	}
}

func (wrappa ConcurrencyLimitsWrappa) Wrap(handlers rata.Handlers) rata.Handlers {
	wrapped := rata.Handlers{}

	for _, limit := range wrappa.concurrentRequestLimits {
		for name, handler := range handlers {
			if limit.Action == name {
				wrapped[name] = wrapHandler(
					wrappa.logger,
					limit.Limit,
					handler,
				)
			} else {
				wrapped[name] = handler
			}
		}
	}

	return wrapped
}

func wrapHandler(logger lager.Logger, limit int, handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	})
}
