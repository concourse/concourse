package triggerbuild

import (
	"fmt"
	"log"
	"net/http"

	"github.com/tedsuo/router"

	"github.com/concourse/atc/builds"
	"github.com/concourse/atc/config"
	"github.com/concourse/atc/queue"
	"github.com/concourse/atc/server/routes"
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

	build, err := handler.queuer.Trigger(job)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, "error building: %s", err)
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
