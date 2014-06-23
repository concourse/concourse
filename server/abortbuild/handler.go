package abortbuild

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/pivotal-golang/lager"
	"github.com/tedsuo/router"

	"github.com/concourse/atc/config"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/server/routes"
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

	handler.logger.Info("web", "aborting", "", lager.Data{
		"job":   job.Name,
		"build": buildID,
	})

	build, err := handler.db.GetBuild(job.Name, buildID)
	if err != nil {
		handler.logger.Error("web", "get-build-failed", "", err, lager.Data{
			"job":   job.Name,
			"build": buildID,
		})

		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	err = handler.db.AbortBuild(job.Name, buildID)
	if err != nil {
		handler.logger.Error("web", "abort-build-failed", "database", err, lager.Data{
			"job":   job.Name,
			"build": build.ID,
		})

		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if build.AbortURL != "" {
		resp, err := handler.httpClient.Post(build.AbortURL, "", nil)
		if err != nil {
			handler.logger.Error("web", "abort-build-failed", "abort url", err, lager.Data{
				"job":   job.Name,
				"build": build.ID,
			})

			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		resp.Body.Close()
	}

	redirectPath, err := routes.Routes.PathForHandler(routes.GetBuild, router.Params{
		"job":   job.Name,
		"build": fmt.Sprintf("%d", build.ID),
	})
	if err != nil {
		handler.logger.Fatal("web", "create-redirect-uri-failed", "", err, lager.Data{
			"job":   job.Name,
			"build": build.ID,
		})
	}

	http.Redirect(w, r, redirectPath, 302)
}
