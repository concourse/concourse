package wrappa

import (
	"net/http"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/metric"
)

type APIMetricsWrappa struct {
	logger lager.Logger
}

func NewAPIMetricsWrappa(logger lager.Logger) Wrappa {
	return APIMetricsWrappa{
		logger: logger,
	}
}

func (wrappa APIMetricsWrappa) Wrap(handlers map[string]http.Handler) map[string]http.Handler {
	wrapped := map[string]http.Handler{}

	for name, handler := range handlers {
		switch name {
		case atc.BuildEvents, atc.DownloadCLI, atc.HijackContainer:
			wrapped[name] = handler
		default:
			wrapped[name] = metric.WrapHandler(
				wrappa.logger,
				metric.Metrics,
				name,
				handler,
			)
		}
	}

	return wrapped
}
