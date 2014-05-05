package triggerbuild

import (
	"log"
	"net/http"

	"github.com/winston-ci/winston/builder"
	"github.com/winston-ci/winston/jobs"
)

type handler struct {
	jobs    map[string]jobs.Job
	builder builder.Builder
}

func NewHandler(jobs map[string]jobs.Job, builder builder.Builder) http.Handler {
	return &handler{
		jobs:    jobs,
		builder: builder,
	}
}

func (handler *handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	job, found := handler.jobs[r.FormValue(":job")]
	if !found {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	log.Println("triggering", job)

	_, err := handler.builder.Build(job)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
}
