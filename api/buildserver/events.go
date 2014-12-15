package buildserver

import (
	"net/http"
	"strconv"

	"github.com/concourse/atc/auth"
	"github.com/concourse/atc/event"
)

func (s *Server) BuildEvents(w http.ResponseWriter, r *http.Request) {
	buildIDStr := r.FormValue(":build_id")

	buildID, err := strconv.Atoi(buildIDStr)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	build, err := s.db.GetBuild(buildID)
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	var censor event.Censor
	if !s.fallback.IsAuthenticated(r) {
		censor = auth.Censor

		if build.OneOff() {
			auth.Unauthorized(w)
			return
		}

		config, err := s.configDB.GetConfig()
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		public, err := config.JobIsPublic(build.JobName)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		if !public {
			auth.Unauthorized(w)
			return
		}
	}

	streamDone := make(chan struct{})

	go func() {
		defer close(streamDone)
		s.eventHandlerFactory(s.db, buildID, s.engine, censor).ServeHTTP(w, r)
	}()

	select {
	case <-streamDone:
	case <-s.drain:
	}
}
