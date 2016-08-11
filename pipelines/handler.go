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

func (pdbh *PipelineHandlerFactory) HandlerFor(pipelineScopedHandler func(db.PipelineDB) http.Handler, allowPublic bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
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

		pipelineDB := pdbh.pipelineDBFactory.Build(savedPipeline)
		if auth.IsAuthorized(r) || (allowPublic && pipelineDB.IsPublic()) {
			pipelineScopedHandler(pipelineDB).ServeHTTP(w, r)
			return
		}

		if auth.IsAuthenticated(r) {
			pdbh.rejector.Forbidden(w, r)
			return
		}

		pdbh.rejector.Unauthorized(w, r)
	}
}
