package wrappa

import (
	"net/http"

	"code.cloudfoundry.org/lager"
	"github.com/NYTimes/gziphandler"
	"github.com/concourse/concourse/atc"
)

type CompressionWrappa struct {
	lager.Logger
}

func NewCompressionWrappa(logger lager.Logger) Wrappa {
	return CompressionWrappa{
		logger,
	}
}

func (wrappa CompressionWrappa) Wrap(handlers map[string]http.Handler) map[string]http.Handler {
	wrapped := map[string]http.Handler{}

	for name, handler := range handlers {
		switch name {
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
