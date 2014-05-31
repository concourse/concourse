package triggerbuild

import (
	"fmt"
	"log"
	"net/http"

	"github.com/tedsuo/router"

	"github.com/winston-ci/winston/builder"
	"github.com/winston-ci/winston/config"
	"github.com/winston-ci/winston/server/routes"
)

type handler struct {
	jobs    config.Jobs
	builder builder.Builder
}

func NewHandler(jobs config.Jobs, builder builder.Builder) http.Handler {
	return &handler{
		jobs:    jobs,
		builder: builder,
	}
}

func (handler *handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	job, found := handler.jobs.Lookup(r.FormValue(":job"))
	if !found {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	log.Println("triggering", job)

	build, err := handler.builder.Build(job, nil)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	redirectPath, err := routes.Routes.PathForHandler(routes.GetBuild, router.Params{
		"job":   job.Name,
		"build": fmt.Sprintf("%d", build.ID),
	})
	if err != nil {
		log.Fatalln("failed to construct redirect uri:", err)
	}

	http.Redirect(w, r, redirectPath, 302)
}
