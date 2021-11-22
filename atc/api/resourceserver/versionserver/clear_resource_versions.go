package versionserver

import (
	"code.cloudfoundry.org/lager"
	"fmt"
	"github.com/concourse/concourse/atc/db"
	"github.com/google/jsonapi"
	"net/http"
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
			logger.Debug("could-not-find-resource", lager.Data{
				"resource": resourceName,
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

		resourceConfigScopeID := resource.ResourceConfigScopeID()
		// When a resource is first created it does not immediately have a resourceConfigScopeID
		if resourceConfigScopeID == 0 {
			w.Header().Set("Content-Type", jsonapi.MediaType)
			w.WriteHeader(http.StatusInternalServerError)
			_ = jsonapi.MarshalErrors(w, []*jsonapi.ErrorObject{{
				Title:  "Resource Not Connected To A Scope Yet",
				Detail: fmt.Sprintf("Resource Config Scope ID does not exist for '%s'", resourceName),
				Status: "500",
			}})
			return
		}
		resourceConfigScope, found, err := s.resourceConfigScopeFactory.FindResourceConfigScopeByID(resourceConfigScopeID)

		if err != nil {
			logger.Error("failed-to-get-resource-config-scope", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		if !found {
			logger.Debug("could-not-find-resource-config-scope", lager.Data{
				"resource": resourceName,
				"resource_config_scope_id": resourceConfigScopeID,
			})
			w.Header().Set("Content-Type", jsonapi.MediaType)
			w.WriteHeader(http.StatusNotFound)
			_ = jsonapi.MarshalErrors(w, []*jsonapi.ErrorObject{{
				Title:  "Resource Config Scope ID Not Found Error",
				Detail: fmt.Sprintf("Resource Config Scope with ID '%d' for name '%s' not found.", resourceConfigScopeID, resourceName),
				Status: "404",
			}})
			return
		}

		resourcesWithScope := resource

		return
	})
}
