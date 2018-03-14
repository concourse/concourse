package buildserver

import (
	"net/http"
	"time"

	"github.com/concourse/atc"
	"github.com/concourse/atc/db"

	"code.cloudfoundry.org/lager"
)

func (s *Server) ReadOutputFromBuildPlan(build db.Build) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		logger := s.logger.Session("read-output", lager.Data{
			"build": build.ID(),
		})

		planID := atc.PlanID(r.FormValue(":plan_id"))
		if len(planID) == 0 {
			logger.Info("no-plan-id-specified")
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		logger.Debug("reading-output", lager.Data{"plan": planID})

		cn := w.(http.CloseNotifier).CloseNotify()

		for build.Tracker() == "" {
			found, err := build.Reload()
			if err != nil {
				logger.Error("failed-to-reload-build", err)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}

			if !found {
				logger.Info("build-disappeared")
				w.WriteHeader(http.StatusNotFound)
				return
			}

			select {
			case <-time.After(time.Second):
			case <-cn:
				return
			}
		}

		if build.Tracker() == s.peerURL {
			engineBuild, err := s.engine.LookupBuild(logger, build)
			if err != nil {
				logger.Error("failed-to-lookup-build", err)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}

			w.WriteHeader(http.StatusOK)

			engineBuild.SendOutput(logger, planID, w)
		} else {
			logger.Debug("forwarding", lager.Data{"to": build.Tracker()})

			err := s.forwardRequest(w, r, build.Tracker(), atc.ReadOutputFromBuildPlan)
			if err != nil {
				logger.Error("failed-to-forward-request", err)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
		}
	})
}
