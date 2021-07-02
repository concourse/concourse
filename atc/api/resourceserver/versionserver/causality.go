package versionserver

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"code.cloudfoundry.org/lager"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db"
)

func (s *Server) GetDownstreamResourceCausality(pipeline db.Pipeline) http.Handler {
	return s.getResourceCausality(db.CausalityDownstream, pipeline)
}

func (s *Server) GetUpstreamResourceCausality(pipeline db.Pipeline) http.Handler {
	return s.getResourceCausality(db.CausalityUpstream, pipeline)
}

func (s *Server) getResourceCausality(direction db.CausalityDirection, pipeline db.Pipeline) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		logger := s.logger.Session(fmt.Sprintf("%v-causality", direction))

		if !atc.EnableResourceCausality {
			logger.Info("causality-disabled")
			w.WriteHeader(http.StatusForbidden)
			return
		}

		versionIDString := r.FormValue(":resource_config_version_id")
		resourceName := r.FormValue(":resource_name")
		versionID, _ := strconv.Atoi(versionIDString)

		resource, found, err := pipeline.Resource(resourceName)
		if err != nil {
			logger.Error("failed-to-get-resource", err, lager.Data{"resource-name": resourceName})
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		if !found {
			logger.Info("resource-not-found", lager.Data{"resource-name": resourceName})
			w.WriteHeader(http.StatusNotFound)
			return
		}

		causality, found, err := resource.Causality(versionID, direction)
		if err != nil {
			if err == db.ErrTooManyBuilds || err == db.ErrTooManyResourceVersions {
				logger.Error("too-many-nodes", err, lager.Data{"resource-name": resourceName, "resource-config-version": versionID})
				w.WriteHeader(http.StatusUnprocessableEntity)
				return
			} else {
				logger.Error("failed-to-fetch", err, lager.Data{"resource-name": resourceName, "resource-config-version": versionID})
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
		}

		if !found {
			logger.Info("resource-version-not-found", lager.Data{"resource-name": resourceName, "resource-config-version": versionID})
			w.WriteHeader(http.StatusNotFound)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		err = json.NewEncoder(w).Encode(causality)
		if err != nil {
			logger.Error("failed-to-encode", err, lager.Data{"resource-name": resourceName, "resource-config-version": versionID})
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	})
}
