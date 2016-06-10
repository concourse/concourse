package buildserver

import (
	"net/http"
	"strconv"

	"github.com/concourse/atc/auth"
)

func (s *Server) BuildEvents(w http.ResponseWriter, r *http.Request) {
	buildIDStr := r.FormValue(":build_id")

	buildID, err := strconv.Atoi(buildIDStr)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	teamDB := s.teamDBFactory.GetTeamDB(getTeamName(r))
	build, found, err := teamDB.GetBuild(buildID)
	if err != nil {
		s.logger.Error("failed-to-get-build", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if !found {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	if !auth.IsAuthenticated(r) {
		if build.IsOneOff() {
			s.rejector.Unauthorized(w, r)
			return
		}

		config, _, err := build.GetConfig()
		if err != nil {
			s.logger.Error("failed-to-get-config", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		public, err := config.JobIsPublic(build.JobName())
		if err != nil {
			s.logger.Error("failed-to-see-job-is-public", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		if !public {
			s.rejector.Unauthorized(w, r)
			return
		}
	}

	streamDone := make(chan struct{})

	go func() {
		defer close(streamDone)

		s.eventHandlerFactory(s.logger, build).ServeHTTP(w, r)
	}()

	select {
	case <-streamDone:
	case <-s.drain:
	}
}
