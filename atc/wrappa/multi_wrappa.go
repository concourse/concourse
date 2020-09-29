package wrappa

import "net/http"

type MultiWrappa []Wrappa

func (wrappas MultiWrappa) Wrap(handlers map[string]http.Handler) map[string]http.Handler {
	for _, w := range wrappas {
		handlers = w.Wrap(handlers)
	}

	return handlers
}
