package wrappa

import "github.com/tedsuo/rata"

type ConcourseVersionWrappa struct {
	version string
}

func NewConcourseVersionWrappa(version string) Wrappa {
	return ConcourseVersionWrappa{
		version: version,
	}
}

func (wrappa ConcourseVersionWrappa) Wrap(handlers rata.Handlers) rata.Handlers {
	wrapped := rata.Handlers{}

	for name, handler := range handlers {
		wrapped[name] = VersionedHandler{
			Version: wrappa.version,
			Handler: handler,
		}
	}

	return wrapped
}
