package buildserver

import (
	"net/http"
	"strconv"

	"github.com/concourse/atc"
	"github.com/concourse/atc/auth"
	"github.com/concourse/atc/db"
)

func (s *Server) BuildEvents(pipelineDB db.PipelineDB) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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

		censor := false
		if !s.fallback.IsAuthenticated(r) {
			censor = true

			if build.OneOff() {
				auth.Unauthorized(w)
				return
			}

			config, _, err := s.configDB.GetConfig(atc.DefaultPipelineName)

			if err != nil {
				s.logger.Error("failed-to-get-config", err)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}

			public, err := config.JobIsPublic(build.JobName)
			if err != nil {
				s.logger.Error("failed-to-see-job-is-public", err)
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
			s.eventHandlerFactory(s.db, buildID, censor).ServeHTTP(w, r)
		}()

		select {
		case <-streamDone:
		case <-s.drain:
		}
	})
}
