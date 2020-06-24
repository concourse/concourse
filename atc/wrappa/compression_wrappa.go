package wrappa

import (
	"net/http"

	"code.cloudfoundry.org/lager"
	"github.com/NYTimes/gziphandler"
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/api/stream"
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
			// watchable endpoints should set MinSize to 0 iff "Accept: text/event-stream" is provided
			wrapped[name] = alwaysGzipIfEventStreamRequestedHandler{handler}
		}
	}

	return wrapped
}

type alwaysGzipIfEventStreamRequestedHandler struct {
	handler http.Handler
}

func (h alwaysGzipIfEventStreamRequestedHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var gzipHandlerFactory func(http.Handler) http.Handler
	if stream.IsRequested(r) {
		// GzipHandlerWithOpts only errors if MinSize < 0 or compression level is OOB
		// Since we're using a static configuration, there's no use in handling the error
		gzipHandlerFactory, _ = gziphandler.GzipHandlerWithOpts(gziphandler.MinSize(0))
	} else {
		gzipHandlerFactory = gziphandler.GzipHandler
	}
	gzipHandlerFactory(h.handler).ServeHTTP(w, r)
}
