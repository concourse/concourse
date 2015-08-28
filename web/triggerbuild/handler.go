package triggerbuild

import (
	"fmt"
	"net/http"

	"github.com/pivotal-golang/lager"
	"github.com/tedsuo/rata"

	"github.com/concourse/atc/db"
	"github.com/concourse/atc/pipelines"
	"github.com/concourse/atc/web/routes"
)

type server struct {
	logger                lager.Logger
	radarSchedulerFactory pipelines.RadarSchedulerFactory
}

func NewServer(
	logger lager.Logger,
	radarSchedulerFactory pipelines.RadarSchedulerFactory,
) *server {
	return &server{
		logger:                logger,
		radarSchedulerFactory: radarSchedulerFactory,
	}
}

func (server *server) TriggerBuild(pipelineDB db.PipelineDB) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		config, _, err := pipelineDB.GetConfig()
		if err != nil {
			server.logger.Error("failed-to-load-config", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		job, found := config.Jobs.Lookup(r.FormValue(":job"))
		if !found {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		log := server.logger.Session("trigger-build", lager.Data{
			"job": job.Name,
		})

		log.Debug("triggering")

		scheduler := server.radarSchedulerFactory.BuildScheduler(pipelineDB)

		build, _, err := scheduler.TriggerImmediately(log, job, config.Resources)
		if err != nil {
			log.Error("failed-to-trigger", err)
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintf(w, "failed to trigger: %s", err)
			return
		}

		redirectPath, err := routes.Routes.CreatePathForRoute(routes.GetBuild, rata.Params{
			"pipeline_name": pipelineDB.GetPipelineName(),
			"job":           job.Name,
			"build":         build.Name,
		})
		if err != nil {
			log.Fatal("failed-to-construct-redirect-uri", err, lager.Data{
				"pipeline": pipelineDB.GetPipelineName(),
				"job":      job.Name,
				"build":    build.Name,
			})
		}

		http.Redirect(w, r, redirectPath, http.StatusFound)
	})
}
