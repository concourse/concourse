package buildserver

import (
	"net/http"
	"strconv"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/atc/auth"
	"github.com/concourse/atc/db"
)

type scopedHandlerFactory struct {
	logger        lager.Logger
	teamDBFactory db.TeamDBFactory
	rejector      auth.Rejector
}

func NewScopedHandlerFactory(
	logger lager.Logger,
	teamDBFactory db.TeamDBFactory,
) *scopedHandlerFactory {
	return &scopedHandlerFactory{
		logger:        logger,
		teamDBFactory: teamDBFactory,
		rejector:      auth.UnauthorizedRejector{},
	}
}

func (f *scopedHandlerFactory) HandlerFor(buildScopedHandler func(db.Build) http.Handler, allowPrivateJob bool) http.HandlerFunc {
	logger := f.logger.Session("scoped-build-factory")

	return func(w http.ResponseWriter, r *http.Request) {
		buildIDStr := r.FormValue(":build_id")

		buildID, err := strconv.Atoi(buildIDStr)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		teamDB := f.teamDBFactory.GetTeamDB(auth.GetAuthTeamName(r))
		build, found, err := teamDB.GetBuild(buildID)
		if err != nil {
			logger.Error("failed-to-get-build", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		if found {
			hasAccess, err := f.verifyBuildAcccess(logger, build, r, allowPrivateJob)
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}

			if !hasAccess {
				f.rejector.Unauthorized(w, r)
				return
			}
		} else {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		buildScopedHandler(build).ServeHTTP(w, r)
	}
}

func (f *scopedHandlerFactory) verifyBuildAcccess(logger lager.Logger, build db.Build, r *http.Request, allowPrivateJob bool) (bool, error) {
	if !auth.IsAuthenticated(r) {
		if build.IsOneOff() {
			return false, nil
		}

		pipeline, err := build.GetPipeline()
		if err != nil {
			f.logger.Error("failed-to-get-pipeline", err)
			return false, err
		}

		if !pipeline.Public {
			return false, nil
		}

		if !allowPrivateJob {
			config, _, err := build.GetConfig()
			if err != nil {
				f.logger.Error("failed-to-get-config", err)
				return false, err
			}

			public, err := config.JobIsPublic(build.JobName())
			if err != nil {
				f.logger.Error("failed-to-see-job-is-public", err)
				return false, err
			}

			if !public {
				return false, nil
			}
		}
	}

	return true, nil
}
