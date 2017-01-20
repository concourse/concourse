package versionserver

import (
	"net/http"
	"strconv"

	"github.com/concourse/atc/db"
	"github.com/concourse/atc/dbng"
	"github.com/tedsuo/rata"
)

func (s *Server) EnableResourceVersion(pipelineDB db.PipelineDB, _ dbng.Pipeline) http.Handler {
	logger := s.logger.Session("enable-resource-version")
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resourceID, err := strconv.Atoi(rata.Param(r, "resource_version_id"))
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		err = pipelineDB.EnableVersionedResource(resourceID)
		if err != nil {
			logger.Error("failed-to-enable-versioned-resource", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
	})
}
