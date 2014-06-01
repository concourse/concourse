package triggerbuild

import (
	"fmt"
	"log"
	"net/http"

	"github.com/tedsuo/router"

	"github.com/winston-ci/winston/builds"
	"github.com/winston-ci/winston/config"
	"github.com/winston-ci/winston/queue"
	"github.com/winston-ci/winston/server/routes"
)

type handler struct {
	jobs   config.Jobs
	queuer queue.Queuer
}

func NewHandler(jobs config.Jobs, queuer queue.Queuer) http.Handler {
	return &handler{
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

	log.Println("triggering", job)

	var build builds.Build

	startedBuild, buildErr := handler.queuer.Trigger(job)

	select {
	case build = <-startedBuild:
		redirectPath, err := routes.Routes.PathForHandler(routes.GetBuild, router.Params{
			"job":   job.Name,
			"build": fmt.Sprintf("%d", build.ID),
		})
		if err != nil {
			log.Fatalln("failed to construct redirect uri:", err)
		}

		http.Redirect(w, r, redirectPath, 302)
	case err := <-buildErr:
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintf(w, "error building: %s", err)
			return
		}
	}
}
