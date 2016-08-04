package wrappa

import (
	"github.com/concourse/atc/metric"
	"github.com/concourse/atc/web"
	"code.cloudfoundry.org/lager"
	"github.com/tedsuo/rata"
)

type WebMetricsWrappa struct {
	logger lager.Logger
}

func NewWebMetricsWrappa(logger lager.Logger) Wrappa {
	return WebMetricsWrappa{
		logger: logger,
	}
}

func (wrappa WebMetricsWrappa) Wrap(handlers rata.Handlers) rata.Handlers {
	wrapped := rata.Handlers{}

	for name, handler := range handlers {
		switch name {
		case web.Public:
			wrapped[name] = handler
		default:
			wrapped[name] = metric.WrapHandler(wrappa.logger, name, handler)
		}
	}

	return wrapped
}
