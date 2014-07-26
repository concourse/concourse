package triggerbuild

import (
	"fmt"
	"net/http"

	"github.com/concourse/atc/builder"
	"github.com/concourse/atc/db"
	"github.com/pivotal-golang/lager"
	"github.com/tedsuo/rata"

	"github.com/concourse/atc/config"
	"github.com/concourse/atc/server/routes"
)

type handler struct {
	logger lager.Logger

	jobs config.Jobs

	db      db.DB
	builder builder.Builder
}

func NewHandler(
	logger lager.Logger,
	jobs config.Jobs,
	db db.DB,
	builder builder.Builder,
) http.Handler {
	return &handler{
		logger: logger,

		jobs: jobs,

		db:      db,
		builder: builder,
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

	build, err := handler.db.CreateBuild(job.Name)
	if err != nil {
		log.Error("failed-to-create-build", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	err = handler.builder.Build(build, job, nil)
	if err != nil {
		log.Error("triggering-failed", err)
		w.WriteHeader(http.StatusInternalServerError)
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
