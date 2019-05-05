package wrappa

import (
	"code.cloudfoundry.org/lager"
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/metrics"
	"github.com/tedsuo/rata"
)

type APIMetricsWrappa struct {
	logger lager.Logger
}

func NewAPIMetricsWrappa(logger lager.Logger) Wrappa {
	return APIMetricsWrappa{
		logger: logger,
	}
}

func (wrappa APIMetricsWrappa) Wrap(handlers rata.Handlers) rata.Handlers {
	wrapped := rata.Handlers{}

	for name, handler := range handlers {
		switch name {
		case atc.BuildEvents, atc.DownloadCLI, atc.HijackContainer:
			wrapped[name] = handler
		default:
			wrapped[name] = metrics.WrapHandler(handler)
		}
	}

	return wrapped
}
