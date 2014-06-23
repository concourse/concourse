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

	handler.logger.Info("trigger-build", "triggering", "", lager.Data{
		"job": job.Name,
	})

	var build builds.Build

	build, err := handler.queuer.Trigger(job)
	if err != nil {
		handler.logger.Error("trigger-build", "triggering-failed", "", err, lager.Data{
			"job": job.Name,
		})

		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	redirectPath, err := routes.Routes.PathForHandler(routes.GetBuild, router.Params{
		"job":   job.Name,
		"build": fmt.Sprintf("%d", build.ID),
	})
	if err != nil {
		handler.logger.Fatal("trigger-build", "constructing-redirect-uri-failed", "", err, lager.Data{
			"job":   job.Name,
			"build": build.ID,
		})
	}

	http.Redirect(w, r, redirectPath, 302)
}
