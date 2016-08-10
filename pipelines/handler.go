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
		authorized, response := auth.IsAuthorized(r)
		if !allowPublic && !authorized {
			if response == auth.Unauthorized {
				pdbh.rejector.Unauthorized(w, r)
			} else if response == auth.Forbidden {
				pdbh.rejector.Forbidden(w, r)
			}
			return
		}

		pipelineName := r.FormValue(":pipeline_name")
		teamName := r.FormValue(":team_name")
		teamDB := pdbh.teamDBFactory.GetTeamDB(teamName)
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

		if !authorized && !pipelineDB.IsPublic() {
			if response == auth.Unauthorized {
				pdbh.rejector.Unauthorized(w, r)
			} else if response == auth.Forbidden {
				pdbh.rejector.Forbidden(w, r)
			}
			return
		}

		pipelineScopedHandler(pipelineDB).ServeHTTP(w, r)
	}
}
