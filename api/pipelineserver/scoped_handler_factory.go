package pipelineserver

import (
	"net/http"

	"github.com/concourse/atc/auth"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/dbng"
)

type ScopedHandlerFactory struct {
	pipelineDBFactory db.PipelineDBFactory
	teamDBFactory     db.TeamDBFactory
	pipelineFactory   dbng.PipelineFactory
}

func NewScopedHandlerFactory(
	pipelineDBFactory db.PipelineDBFactory,
	teamDBFactory db.TeamDBFactory,
	pipelineFactory dbng.PipelineFactory,
) *ScopedHandlerFactory {
	return &ScopedHandlerFactory{
		pipelineDBFactory: pipelineDBFactory,
		teamDBFactory:     teamDBFactory,
		pipelineFactory:   pipelineFactory,
	}
}

func (pdbh *ScopedHandlerFactory) HandlerFor(pipelineScopedHandler func(db.PipelineDB, dbng.Pipeline) http.Handler) http.HandlerFunc {
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

		dbPipeline := pdbh.pipelineFactory.GetPipelineByID(pipelineDB.TeamID(), pipelineDB.Pipeline().ID)

		pipelineScopedHandler(pipelineDB, dbPipeline).ServeHTTP(w, r)
	}
}
