package resourceserver

import (
	"encoding/json"
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

		//TODO: Validate that the token is mapped to the requested team associated with the resource
		// using the pipelineDB that is passed into this method for that effort
		scanner := s.scannerFactory.NewResourceScanner(pipelineDB, dbPipeline)

		//TODO: Add better messaging and error codes
		err := scanner.ScanFromVersion(logger, resourceName, nil)
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
