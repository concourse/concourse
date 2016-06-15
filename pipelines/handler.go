package pipelines

import (
	"database/sql"
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
		if !allowPublic && !auth.IsAuthorized(r) {
			pdbh.rejector.Unauthorized(w, r)
			return
		}

		pipelineName := r.FormValue(":pipeline_name")
		teamName := r.FormValue(":team_name")
		teamDB := pdbh.teamDBFactory.GetTeamDB(teamName)
		savedPipeline, err := teamDB.GetPipelineByName(pipelineName)
		if err != nil {
			if err == sql.ErrNoRows {
				w.WriteHeader(http.StatusNotFound)
			} else {
				w.WriteHeader(http.StatusInternalServerError)
			}
			return
		}
		pipelineDB := pdbh.pipelineDBFactory.Build(savedPipeline)

		if !auth.IsAuthorized(r) && !pipelineDB.IsPublic() {
			pdbh.rejector.Unauthorized(w, r)
			return
		}

		pipelineScopedHandler(pipelineDB).ServeHTTP(w, r)
	}
}
