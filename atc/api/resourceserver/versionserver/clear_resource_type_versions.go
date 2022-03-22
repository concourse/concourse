package versionserver

import (
	"net/http"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db"
)

func (s *Server) ClearResourceTypeVersions(pipeline db.Pipeline) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resourceTypeName := r.FormValue(":resource_type_name")
		logger := s.logger.Session("clear-resource-type-versions", lager.Data{"resource-type-name": resourceTypeName})

		resourceType, found, err := pipeline.ResourceType(resourceTypeName)
		if err != nil {
			logger.Error("failed-to-get-resource-type", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		if !found {
			logger.Debug("resource-type-not-found")
			w.WriteHeader(http.StatusNotFound)
			return
		}

		versionsDeleted, err := resourceType.ClearVersions()
		if err != nil {
			logger.Error("failed-to-clear-versions", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		s.writeJSONResponse(w, atc.ClearVersionsResponse{VersionsRemoved: versionsDeleted})
	})
}
