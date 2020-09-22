package wrappa

import "github.com/tedsuo/rata"

type MultiWrappa []Wrappa

func (wrappas MultiWrappa) Wrap(handlers rata.Handlers) rata.Handlers {
	for i := len(wrappas) - 1; i >= 0; i-- {
		handlers = wrappas[i].Wrap(handlers)
	}

	return handlers
}
