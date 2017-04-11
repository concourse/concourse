package resourceserver

import (
	"encoding/json"
	"fmt"
	"net/http"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/dbng"
	"github.com/concourse/atc/resource"
	"github.com/tedsuo/rata"
)

// CheckResourceWebHook defines a handler for process a check resource request via an access token.
func (s *Server) CheckResourceWebHook(pipelineDB db.PipelineDB, dbPipeline dbng.Pipeline) http.Handler {
	logger := s.logger.Session("check-resource-webhook")

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resourceName := rata.Param(r, "resource_name")
		accessToken := r.URL.Query().Get("access_token")

		if accessToken == "" {
			logger.Info("malformed-request", lager.Data{"error": "missing access_token"})
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		pipelineResource, found, err := pipelineDB.GetResource(resourceName)
		if !found {
			logger.Info("resource-not-found", lager.Data{"error": fmt.Sprintf("Resource not found %s", resourceName)})
			w.WriteHeader(http.StatusNotFound)
			return
		}

		if err != nil {
			logger.Info("database-error", lager.Data{"error": err})
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		token := pipelineResource.Config.Source["webhook_token"]
		if token != accessToken {
			logger.Info("invalid-token-error", lager.Data{"error": fmt.Sprintf("Actual token %s does not match expected token %s", accessToken, token)})
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		scanner := s.scannerFactory.NewResourceScanner(pipelineDB, dbPipeline)
		err = scanner.ScanFromVersion(logger, resourceName, nil)
		switch scanErr := err.(type) {
		case resource.ErrResourceScriptFailed:
			checkResponseBody := atc.CheckResponseBody{
				ExitStatus: scanErr.ExitStatus,
				Stderr:     scanErr.Stderr,
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(checkResponseBody)
		case db.ResourceNotFoundError:
			w.WriteHeader(http.StatusNotFound)
		case error:
			w.WriteHeader(http.StatusInternalServerError)
		default:
			w.WriteHeader(http.StatusOK)
		}
	})
}
