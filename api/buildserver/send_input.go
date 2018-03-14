package buildserver

import (
	"io"
	"net/http"
	"time"

	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
	"github.com/tedsuo/rata"

	"code.cloudfoundry.org/lager"
)

func (s *Server) SendInputToBuildPlan(build db.Build) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		logger := s.logger.Session("send-input", lager.Data{
			"build": build.ID(),
		})

		planID := atc.PlanID(r.FormValue(":plan_id"))
		if len(planID) == 0 {
			logger.Info("no-plan-id-specified")
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		logger.Debug("sending-input", lager.Data{"plan": planID})

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

			engineBuild.ReceiveInput(logger, planID, r.Body)

			w.WriteHeader(http.StatusNoContent)
		} else {
			logger.Debug("forwarding", lager.Data{"to": build.Tracker()})

			err := s.forwardRequest(w, r, build.Tracker(), atc.SendInputToBuildPlan)
			if err != nil {
				logger.Error("failed-to-forward-request", err)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
		}
	})
}

func (s *Server) forwardRequest(w http.ResponseWriter, r *http.Request, host string, route string) error {
	generator := rata.NewRequestGenerator(host, atc.Routes)

	req, err := generator.CreateRequest(
		route,
		rata.Params{
			"build_id": r.FormValue(":build_id"),
			"plan_id":  r.FormValue(":plan_id"),
		},
		r.Body,
	)
	if err != nil {
		return err
	}

	req.Header = r.Header

	client := &http.Client{
		Transport: &http.Transport{
			DisableKeepAlives: true,
		},
	}

	response, err := client.Do(req)
	if err != nil {
		return err
	}

	w.WriteHeader(response.StatusCode)

	_, err = io.Copy(w, response.Body)
	if err != nil {
		return err
	}

	return nil
}
