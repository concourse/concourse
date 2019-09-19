package wrappa

import (
	"code.cloudfoundry.org/lager"
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
		wrapped[name] = CompressionHandler{
			Handler: handler,
			Logger:  wrappa.Logger,
		}
	}

	return wrapped
}
