package versionserver

import (
	"net/http"
	"strconv"

	"code.cloudfoundry.org/lager/v3"
	"github.com/bytedance/sonic"
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/api/present"
	"github.com/concourse/concourse/atc/db"
)

func (s *Server) ListBuildsWithVersionAsOutput(pipeline db.Pipeline) http.Handler {
	logger := s.logger.Session("list-builds-with-version-as-output")

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		versionIDString := r.FormValue(":resource_config_version_id")
		resourceName := r.FormValue(":resource_name")
		versionID, _ := strconv.Atoi(versionIDString)

		resource, found, err := pipeline.Resource(resourceName)
		if err != nil {
			logger.Error("failed-to-find-resource", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		if !found {
			logger.Debug("resource-not-found", lager.Data{"resource-name": resourceName})
			w.WriteHeader(http.StatusNotFound)
			return
		}

		builds, err := pipeline.GetBuildsWithVersionAsOutput(resource.ID(), versionID)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		presentedBuilds := []atc.Build{}
		for _, build := range builds {
			presentedBuilds = append(presentedBuilds, present.Build(build, nil, nil))
		}

		w.Header().Set("Content-Type", "application/json")

		w.WriteHeader(http.StatusOK)

		err = sonic.ConfigDefault.NewEncoder(w).Encode(presentedBuilds)
		if err != nil {
			logger.Error("failed-to-encode-builds", err)
			w.WriteHeader(http.StatusInternalServerError)
		}
	})
}
