package resourceserver

import (
	"code.cloudfoundry.org/lager"
	"encoding/json"
	"fmt"
	"github.com/concourse/concourse/atc"
	"github.com/google/jsonapi"
	"io"
	"net/http"

	"github.com/concourse/concourse/atc/db"
)

func (s *Server) ClearResourceCache(pipeline db.Pipeline) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		logger := s.logger.Session("clear-resource-cache")
		resourceName := r.FormValue(":resource_name")

		resource, found, err := pipeline.Resource(resourceName)
		if err != nil {
			logger.Error("failed-to-get-resource", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		var version atc.VersionDeleteBody
		err = json.NewDecoder(r.Body).Decode(&version)
		switch {
			case err == io.EOF:
				version = atc.VersionDeleteBody{}
			case err != nil:
				logger.Info("malformed-request", lager.Data{"error": err.Error()})
				w.WriteHeader(http.StatusBadRequest)
				return
		}

		if !found {
			logger.Debug("could-not-find-resource", lager.Data{
				"resource":   	  resourceName,
				"version":    	  version,
			})
			w.Header().Set("Content-Type", jsonapi.MediaType)
			w.WriteHeader(http.StatusNotFound)
			_ = jsonapi.MarshalErrors(w, []*jsonapi.ErrorObject{{
				Title:  "Resource Not Found Error",
				Detail: fmt.Sprintf("Resource with name '%s' not found.", resourceName),
				Status: "404",
			}})
			return
		}

		rowsDeleted, err := resource.ClearResourceCache(version.Version)

		if err != nil {
			logger.Error("failed-to-clear-resource-cache", err)
			w.Header().Set("Content-Type", jsonapi.MediaType)
			w.WriteHeader(http.StatusInternalServerError)
			_ = jsonapi.MarshalErrors(w, []*jsonapi.ErrorObject{{
				Title:  "Clear Resource Cache Error",
				Detail: err.Error(),
				Status: "500",
			}})
			return
		}

		s.writeJSONResponse(w, atc.ClearResourceCacheResponse{CachesRemoved: rowsDeleted})
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