package triggerbuild

import (
	"fmt"
	"log"
	"net/http"

	"github.com/tedsuo/router"

	"github.com/winston-ci/winston/builder"
	"github.com/winston-ci/winston/config"
	"github.com/winston-ci/winston/db"
	"github.com/winston-ci/winston/server/routes"
)

type handler struct {
	jobs      config.Jobs
	resources config.Resources
	db        db.DB
	builder   builder.Builder
}

func NewHandler(jobs config.Jobs, resources config.Resources, db db.DB, builder builder.Builder) http.Handler {
	return &handler{
		jobs:      jobs,
		resources: resources,
		db:        db,
		builder:   builder,
	}
}

func (handler *handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	job, found := handler.jobs.Lookup(r.FormValue(":job"))
	if !found {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	log.Println("triggering", job)

	resources := handler.resources
	for name, passed := range job.Inputs {
		if passed == nil {
			continue
		}

		resource, found := resources.Lookup(name)
		if !found {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		outputs, err := handler.db.GetCommonOutputs(passed, name)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		if len(outputs) == 0 {
			w.WriteHeader(http.StatusPreconditionFailed)
			fmt.Fprintf(w, "unsatisfied input: %s; depends on %v\n", name, passed)
			return
		}

		resource.Source = outputs[len(outputs)-1]

		resources = resources.UpdateResource(resource)
	}

	build, err := handler.builder.Build(job, resources)
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
