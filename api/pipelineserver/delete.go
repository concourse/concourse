package pipelineserver

import (
	"net/http"

	"github.com/concourse/atc/db"
	"github.com/pivotal-golang/lager"
)

func (s *Server) DeletePipeline(pipelineDB db.PipelineDB) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		err := pipelineDB.Destroy()
		if err != nil {
			s.logger.Error("destroying-pipeline", err, lager.Data{
				"pipeline-name": pipelineDB.GetPipelineName(),
			})

			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusNoContent)
	})
}
