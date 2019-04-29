package buildserver

import (
	"net/http"

	"github.com/concourse/concourse/atc/db"
)

func (s *Server) BuildEvents(build db.Build) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		streamDone := make(chan struct{})

		go func() {
			defer close(streamDone)

			s.eventHandlerFactory(s.logger, build).ServeHTTP(w, r)
		}()

		select {
		case <-streamDone:
		}
	})
}
