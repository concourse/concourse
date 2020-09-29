package wrappa

import "net/http"

type ConcourseVersionWrappa struct {
	version string
}

func NewConcourseVersionWrappa(version string) Wrappa {
	return ConcourseVersionWrappa{
		version: version,
	}
}

func (wrappa ConcourseVersionWrappa) Wrap(handlers map[string]http.Handler) map[string]http.Handler {
	wrapped := map[string]http.Handler{}

	for name, handler := range handlers {
		wrapped[name] = VersionedHandler{
			Version: wrappa.version,
			Handler: handler,
		}
	}

	return wrapped
}
