package pipelines

import (
	"net/http"

	"github.com/concourse/atc/auth"
	"github.com/concourse/atc/db"
)

type PipelineHandlerFactory struct {
	pipelineDBFactory db.PipelineDBFactory
	teamDBFactory     db.TeamDBFactory
	rejector          auth.Rejector
}

func NewHandlerFactory(
	pipelineDBFactory db.PipelineDBFactory,
	teamDBFactory db.TeamDBFactory,
) *PipelineHandlerFactory {
	return &PipelineHandlerFactory{
		pipelineDBFactory: pipelineDBFactory,
		teamDBFactory:     teamDBFactory,
		rejector:          auth.UnauthorizedRejector{},
	}
}

func (pdbh *PipelineHandlerFactory) HandlerFor(pipelineScopedHandler func(db.PipelineDB) http.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		pipelineDB, ok := r.Context().Value(auth.PipelineDBKey).(db.PipelineDB)
		if !ok {
			pipelineName := r.FormValue(":pipeline_name")
			requestTeamName := r.FormValue(":team_name")

			teamDB := pdbh.teamDBFactory.GetTeamDB(requestTeamName)
			savedPipeline, found, err := teamDB.GetPipelineByName(pipelineName)
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}

			if !found {
				w.WriteHeader(http.StatusNotFound)
				return
			}

			pipelineDB = pdbh.pipelineDBFactory.Build(savedPipeline)
		}

		pipelineScopedHandler(pipelineDB).ServeHTTP(w, r)
	}
}
