package pipelines

import (
	"database/sql"
	"net/http"

	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
)

type PipelineHandlerFactory struct {
	pipelineDBFactory db.PipelineDBFactory
}

func (pdbh *PipelineHandlerFactory) HandlerFor(pipelineScopedHandler func(db.PipelineDB) http.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		pipelineName := r.FormValue(":pipeline_name")
		pipelineDB, err := pdbh.pipelineDBFactory.BuildWithTeamNameAndName(atc.DefaultTeamName, pipelineName)
		if err != nil {
			if err == sql.ErrNoRows {
				w.WriteHeader(http.StatusNotFound)
			} else {
				w.WriteHeader(http.StatusInternalServerError)
			}
			return
		}

		pipelineScopedHandler(pipelineDB).ServeHTTP(w, r)
	}
}

func NewHandlerFactory(pipelineDBFactory db.PipelineDBFactory) *PipelineHandlerFactory {
	return &PipelineHandlerFactory{pipelineDBFactory: pipelineDBFactory}
}
