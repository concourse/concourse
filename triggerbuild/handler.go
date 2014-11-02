package triggerbuild

import (
	"fmt"
	"net/http"

	"github.com/pivotal-golang/lager"
	"github.com/tedsuo/rata"

	"github.com/concourse/atc/db"
	"github.com/concourse/atc/scheduler"
	"github.com/concourse/atc/web/routes"
)

type handler struct {
	logger lager.Logger

	db        db.ConfigDB
	scheduler *scheduler.Scheduler
}

func NewHandler(
	logger lager.Logger,
	db db.ConfigDB,
	scheduler *scheduler.Scheduler,
) http.Handler {
	return &handler{
		logger: logger,

		db:        db,
		scheduler: scheduler,
	}
}

func (handler *handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	config, err := handler.db.GetConfig()
	if err != nil {
		handler.logger.Error("failed-to-load-config", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	job, found := config.Jobs.Lookup(r.FormValue(":job"))
	if !found {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	log := handler.logger.Session("trigger-build", lager.Data{
		"job": job.Name,
	})

	log.Debug("triggering")

	build, err := handler.scheduler.TriggerImmediately(job, config.Resources)
	if err != nil {
		log.Error("failed-to-trigger", err)
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, "failed to trigger: %s", err)
		return
	}

	redirectPath, err := routes.Routes.CreatePathForRoute(routes.GetBuild, rata.Params{
		"job":   job.Name,
		"build": build.Name,
	})
	if err != nil {
		log.Fatal("failed-to-construct-redirect-uri", err, lager.Data{
			"build": build.Name,
		})
	}

	http.Redirect(w, r, redirectPath, http.StatusFound)
}
