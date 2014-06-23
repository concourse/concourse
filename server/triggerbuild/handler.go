package triggerbuild

import (
	"fmt"
	"net/http"

	"github.com/pivotal-golang/lager"
	"github.com/tedsuo/router"

	"github.com/concourse/atc/builds"
	"github.com/concourse/atc/config"
	"github.com/concourse/atc/queue"
	"github.com/concourse/atc/server/routes"
)

type handler struct {
	logger lager.Logger

	jobs   config.Jobs
	queuer queue.Queuer
}

func NewHandler(logger lager.Logger, jobs config.Jobs, queuer queue.Queuer) http.Handler {
	return &handler{
		logger: logger,

		jobs:   jobs,
		queuer: queuer,
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

	var build builds.Build

	build, err := handler.queuer.Trigger(job)
	if err != nil {
		log.Error("triggering-failed", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	redirectPath, err := routes.Routes.PathForHandler(routes.GetBuild, router.Params{
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
