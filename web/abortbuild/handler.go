package abortbuild

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/pivotal-golang/lager"
	"github.com/tedsuo/rata"

	"github.com/concourse/atc/config"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/web/routes"
)

type handler struct {
	logger lager.Logger

	jobs       config.Jobs
	db         db.DB
	httpClient *http.Client
}

func NewHandler(logger lager.Logger, jobs config.Jobs, db db.DB) http.Handler {
	return &handler{
		logger: logger,

		jobs: jobs,
		db:   db,

		httpClient: &http.Client{
			Transport: &http.Transport{
				ResponseHeaderTimeout: 5 * time.Minute,
			},
		},
	}
}

func (handler *handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	job, found := handler.jobs.Lookup(r.FormValue(":job"))
	if !found {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	buildID, err := strconv.Atoi(r.FormValue(":build"))
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	log := handler.logger.Session("abort", lager.Data{
		"job":   job.Name,
		"build": buildID,
	})

	build, err := handler.db.GetBuild(job.Name, buildID)
	if err != nil {
		log.Error("failed-to-get-build", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	err = handler.db.AbortBuild(job.Name, buildID)
	if err != nil {
		log.Error("failed-to-set-aborted", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if build.AbortURL != "" {
		resp, err := handler.httpClient.Post(build.AbortURL, "", nil)
		if err != nil {
			log.Error("failed-to-abort-build", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		resp.Body.Close()
	}

	redirectPath, err := routes.Routes.CreatePathForRoute(routes.GetBuild, rata.Params{
		"job":   job.Name,
		"build": fmt.Sprintf("%d", build.ID),
	})
	if err != nil {
		log.Fatal("failed-to-create-redirect-uri", err)
	}

	http.Redirect(w, r, redirectPath, 302)
}
