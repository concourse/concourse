package wrappa

import "github.com/tedsuo/rata"

type MultiWrappa []Wrappa

func (wrappas MultiWrappa) Wrap(handlers rata.Handlers) rata.Handlers {
	for _, w := range wrappas {
		handlers = w.Wrap(handlers)
	}

	return handlers
}
