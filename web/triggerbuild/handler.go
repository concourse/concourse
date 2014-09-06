package triggerbuild

import (
	"fmt"
	"net/http"

	"github.com/pivotal-golang/lager"
	"github.com/tedsuo/rata"

	"github.com/concourse/atc/config"
	"github.com/concourse/atc/scheduler"
	"github.com/concourse/atc/web/routes"
)

type handler struct {
	logger lager.Logger

	jobs config.Jobs

	scheduler *scheduler.Scheduler
}

func NewHandler(
	logger lager.Logger,
	jobs config.Jobs,
	scheduler *scheduler.Scheduler,
) http.Handler {
	return &handler{
		logger: logger,

		jobs: jobs,

		scheduler: scheduler,
	}
}

func (handler *handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	job, found := handler.jobs.Lookup(r.FormValue(":job"))
	if !found {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	log := handler.logger.Session("trigger-build", lager.Data{
		"job": job.Name,
	})

	log.Debug("triggering")

	build, err := handler.scheduler.TriggerImmediately(job)
	if err != nil {
		log.Error("failed-to-trigger", err)
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, "failed to trigger: %s", err)
		return
	}

	redirectPath, err := routes.Routes.CreatePathForRoute(routes.GetBuild, rata.Params{
		"job":   job.Name,
		"build": fmt.Sprintf("%d", build.ID),
	})
	if err != nil {
		log.Fatal("failed-to-construct-redirect-uri", err, lager.Data{
			"build": build.ID,
		})
	}

	http.Redirect(w, r, redirectPath, 302)
}
