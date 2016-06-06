package buildserver

import (
	"net/http"
	"strconv"

	"github.com/concourse/atc"
	"github.com/concourse/atc/auth"
	"github.com/pivotal-golang/lager"
)

func (s *Server) BuildEvents(w http.ResponseWriter, r *http.Request) {
	buildIDStr := r.FormValue(":build_id")

	buildID, err := strconv.Atoi(buildIDStr)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	teamDB := s.teamDBFactory.GetTeamDB(atc.DefaultTeamName)
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
		if build.OneOff() {
			s.rejector.Unauthorized(w, r)
			return
		}

		config, _, err := s.db.GetConfigByBuildID(build.ID)
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
			s.rejector.Unauthorized(w, r)
			return
		}
	}

	streamDone := make(chan struct{})

	go func() {
		defer close(streamDone)

		build, found, err := teamDB.GetBuild(build.ID)
		if err != nil {
			s.logger.Error("failed-to-get-build", err, lager.Data{"build-id": build.ID})
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		if !found {
			s.logger.Info("build-not-found", lager.Data{"build-id": build.ID})
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		buildDB := s.buildDBFactory.GetBuildDB(build)

		s.eventHandlerFactory(s.logger, buildDB).ServeHTTP(w, r)
	}()

	select {
	case <-streamDone:
	case <-s.drain:
	}
}
