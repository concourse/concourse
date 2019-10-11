package wrappa

import (
	"github.com/honeycombio/libhoney-go"
	"github.com/tedsuo/rata"
)

type HoneycombWrappa struct {
	client *libhoney.Client
}

func NewHoneycombWrappa(client *libhoney.Client) Wrappa {
	return HoneycombWrappa{
		client: client,
	}
}

func (wrappa HoneycombWrappa) Wrap(handlers rata.Handlers) rata.Handlers {
	if wrappa.client == nil {
		return handlers
	}

	wrapped := rata.Handlers{}

	for name, handler := range handlers {
		wrapped[name] = HoneycombHandler{
			Client:  wrappa.client,
			Handler: handler,
		}
	}

	return wrapped
}
