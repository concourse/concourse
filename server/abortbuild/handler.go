package abortbuild

import (
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/concourse/atc/config"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/server/routes"
	"github.com/tedsuo/router"
)

type handler struct {
	jobs       config.Jobs
	db         db.DB
	httpClient *http.Client
}

func NewHandler(jobs config.Jobs, db db.DB) http.Handler {
	return &handler{
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

	log.Println("aborting", job.Name, buildID)

	build, err := handler.db.GetBuild(job.Name, buildID)
	if err != nil {
		log.Println("failed to get build:", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	err = handler.db.AbortBuild(job.Name, buildID)
	if err != nil {
		log.Println("failed to abort build in db:", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if build.AbortURL != "" {
		resp, err := handler.httpClient.Post(build.AbortURL, "", nil)
		if err != nil {
			log.Println("failed to abort build:", err)
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
		log.Fatalln("failed to construct redirect uri:", err)
	}

	http.Redirect(w, r, redirectPath, 302)
}
