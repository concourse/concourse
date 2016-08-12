package pipelineserver

import (
	"net/http"

	"github.com/concourse/atc/auth"
	"github.com/concourse/atc/db"
)

type ScopedHandlerFactory struct {
	pipelineDBFactory db.PipelineDBFactory
	teamDBFactory     db.TeamDBFactory
}

func NewScopedHandlerFactory(
	pipelineDBFactory db.PipelineDBFactory,
	teamDBFactory db.TeamDBFactory,
) *ScopedHandlerFactory {
	return &ScopedHandlerFactory{
		pipelineDBFactory: pipelineDBFactory,
		teamDBFactory:     teamDBFactory,
	}
}

func (pdbh *ScopedHandlerFactory) HandlerFor(pipelineScopedHandler func(db.PipelineDB) http.Handler) http.HandlerFunc {
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
