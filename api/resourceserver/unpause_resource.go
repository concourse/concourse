package resourceserver

import (
	"net/http"

	"github.com/concourse/atc/db"
	"github.com/tedsuo/rata"
)

func (s *Server) UnpauseResource(pipelineDB db.PipelineDB) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resourceName := rata.Param(r, "resource_name")

		err := pipelineDB.UnpauseResource(resourceName)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
	})
}
