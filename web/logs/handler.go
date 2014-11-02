package logs

import (
	"net/http"
	"strconv"

	"github.com/pivotal-golang/lager"

	"github.com/concourse/atc/auth"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/event"
)

func NewHandler(
	logger lager.Logger,
	validator auth.Validator,
	db db.DB,
	configDB db.ConfigDB,
	drain <-chan struct{},
) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		buildIDStr := r.FormValue(":build_id")

		config, err := configDB.GetConfig()
		if err != nil {
			logger.Error("failed-to-load-config", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		log := logger.Session("logs-out", lager.Data{
			"build_id": buildIDStr,
		})

		buildID, err := strconv.Atoi(buildIDStr)
		if err != nil {
			log.Error("invalid-build-id", err)
			return
		}

		authenticated := validator.IsAuthenticated(r)

		var censor event.Censor
		if !authenticated {
			censor = (&auth.EventCensor{}).Censor

			build, err := db.GetBuild(buildID)
			if err != nil {
				log.Error("invalid-build-id", err)
				w.WriteHeader(http.StatusNotFound)
				return
			}

			job, found := config.Jobs.Lookup(build.JobName)
			if !found || !job.Public {
				w.WriteHeader(http.StatusNotFound)
				return
			}
		}

		streamDone := make(chan struct{})

		go func() {
			defer close(streamDone)
			event.NewHandler(db, buildID, censor).ServeHTTP(w, r)
		}()

		select {
		case <-streamDone:
		case <-drain:
		}
	})
}
