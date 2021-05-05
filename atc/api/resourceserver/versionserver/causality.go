package versionserver

import (
	"encoding/json"
	"net/http"
	"strings"

	"code.cloudfoundry.org/lager"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db"
)

// IMPORTANT: This is not yet tested because it is not being used
func (s *Server) GetCausality(pipeline db.Pipeline) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		logger := s.logger.Session("causality")

		resourceName := r.FormValue(":resource_name")

		fields := r.Form["filter"]
		versionFilter := make(atc.Version)
		for _, field := range fields {
			vs := strings.SplitN(field, ":", 2)
			if len(vs) == 2 {
				versionFilter[vs[0]] = vs[1]
			}
		}

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

		// causality, err := pipeline.Causality(versionID)
		causality, err := pipeline.CausalityV2(resource.ID(), versionFilter)
		if err != nil {
			logger.Error("failed-to-fetch", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")

		w.WriteHeader(http.StatusOK)

		_ = json.NewEncoder(w).Encode(causality)
	})
}
