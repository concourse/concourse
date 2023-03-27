package versionserver

import (
	"encoding/json"
	"fmt"
	"net/http"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db"
)

func (s *Server) ClearResourceVersions(pipeline db.Pipeline) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		logger := s.logger.Session("clear-resource-versions")
		resourceName := r.FormValue(":resource_name")

		resource, found, err := pipeline.Resource(resourceName)
		if err != nil {
			logger.Error("failed-to-get-resource", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		if !found {
			logger.Debug("resource-not-found", lager.Data{"resource-name": resourceName})
			w.WriteHeader(http.StatusNotFound)
			return
		}

		versionsDeleted, err := resource.ClearVersions()
		if err != nil {
			logger.Error("failed-to-clear-versions", err)
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(err.Error()))
			return
		}

		s.writeJSONResponse(w, atc.ClearVersionsResponse{VersionsRemoved: versionsDeleted})
	})
}

func (s *Server) writeJSONResponse(w http.ResponseWriter, obj interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	responseJSON, err := json.Marshal(obj)
	if err != nil {
		s.logger.Error("failed-to-marshal-response", err)
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, "failed to generate error response: %s", err)
		return
	}

	_, err = w.Write(responseJSON)
	if err != nil {
		s.logger.Error("failed-to-write-response", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

}
