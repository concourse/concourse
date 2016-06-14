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

	teamDB := s.teamDBFactory.GetTeamDB(auth.GetAuthOrDefaultTeamName(r))
	build, found, err := teamDB.GetBuild(buildID)
	if err != nil {
		s.logger.Error("failed-to-get-build", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if found {
		hasAccess, err := s.verifyBuildAcccess(build, r)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		if !hasAccess {
			s.rejector.Unauthorized(w, r)
			return
		}
	} else {
		w.WriteHeader(http.StatusNotFound)
		return
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
