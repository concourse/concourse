package wrappa

import (
	"code.cloudfoundry.org/lager"
	"github.com/NYTimes/gziphandler"
	"github.com/concourse/concourse/atc"
	"github.com/tedsuo/rata"
)

type CompressionWrappa struct {
	lager.Logger
}

func NewCompressionWrappa(logger lager.Logger) Wrappa {
	return CompressionWrappa{
		logger,
	}
}

func (wrappa CompressionWrappa) Wrap(handlers rata.Handlers) rata.Handlers {
	wrapped := rata.Handlers{}

	for name, handler := range handlers {
		switch atc.RouteAction(name) {
		// always gzip for events
		case atc.BuildEvents:
			gzipEnforcedHandler, err := gziphandler.GzipHandlerWithOpts(gziphandler.MinSize(0))
			if err != nil {
				wrappa.Logger.Error("failed-to-create-gzip-handler", err)
			}

			wrapped[name] = gzipEnforcedHandler(handler)
		// skip gzip as this endpoint does it already
		case atc.DownloadCLI:
			wrapped[name] = handler
		default:
			wrapped[name] = gziphandler.GzipHandler(handler)
		}
	}

	return wrapped
}
