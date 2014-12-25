package abortbuild

import (
	"net/http"
	"strconv"
	"time"

	"github.com/pivotal-golang/lager"
	"github.com/tedsuo/rata"

	"github.com/concourse/atc/db"
	"github.com/concourse/atc/engine"
	"github.com/concourse/atc/web/routes"
)

type handler struct {
	logger lager.Logger

	db     db.DB
	engine engine.Engine

	httpClient *http.Client
}

func NewHandler(logger lager.Logger, db db.DB, engine engine.Engine) http.Handler {
	return &handler{
		logger: logger,

		db:     db,
		engine: engine,

		httpClient: &http.Client{
			Transport: &http.Transport{
				ResponseHeaderTimeout: 5 * time.Minute,
			},
		},
	}
}

func (handler *handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	buildID, err := strconv.Atoi(r.FormValue(":build_id"))
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	log := handler.logger.Session("abort", lager.Data{
		"build": buildID,
	})

	build, err := handler.db.GetBuild(buildID)
	if err != nil {
		log.Error("failed-to-get-build", err)
		w.WriteHeader(http.StatusNotFound)
		return
	}

	engineBuild, err := handler.engine.LookupBuild(build)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	err = engineBuild.Abort()
	if err != nil {
		log.Error("failed-to-unmarshal-metadata", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	redirectPath, err := routes.Routes.CreatePathForRoute(routes.GetBuild, rata.Params{
		"job":   build.JobName,
		"build": build.Name,
	})
	if err != nil {
		log.Fatal("failed-to-create-redirect-uri", err)
	}

	http.Redirect(w, r, redirectPath, 302)
}
