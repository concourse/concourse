package resourceserver

import (
	"net/http"
	"strconv"

	"github.com/concourse/atc/db"
	"github.com/tedsuo/rata"
)

func (s *Server) DisableResourceVersion(pipelineDB db.PipelineDB) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resourceID, err := strconv.Atoi(rata.Param(r, "resource_version_id"))
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		err = pipelineDB.DisableVersionedResource(resourceID)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
	})
}
