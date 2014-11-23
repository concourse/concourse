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

		log := logger.Session("logs-out", lager.Data{
			"build_id": buildIDStr,
		})

		buildID, err := strconv.Atoi(buildIDStr)
		if err != nil {
			log.Error("invalid-build-id", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		build, err := db.GetBuild(buildID)
		if err != nil {
			log.Error("invalid-build-id", err)
			w.WriteHeader(http.StatusNotFound)
			return
		}

		var censor event.Censor
		if !validator.IsAuthenticated(r) {
			censor = (&auth.EventCensor{}).Censor

			if build.OneOff() {
				log.Info("unauthorized-build-event-access")
				auth.Unauthorized(w)
				return
			}

			config, err := configDB.GetConfig()
			if err != nil {
				log.Error("unable-to-get-config", err)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}

			public, err := config.JobIsPublic(build.JobName)
			if err != nil {
				log.Error("unable-to-determine-public-status", err)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}

			if !public {
				log.Info("unauthorized-build-event-access")
				auth.Unauthorized(w)
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
