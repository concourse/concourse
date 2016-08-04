package resourceserver

import (
	"encoding/json"
	"net/http"

	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/resource"
	"code.cloudfoundry.org/lager"
	"github.com/tedsuo/rata"
)

func (s *Server) CheckResource(pipelineDB db.PipelineDB) http.Handler {
	logger := s.logger.Session("check-resource")

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resourceName := rata.Param(r, "resource_name")

		var reqBody atc.CheckRequestBody
		err := json.NewDecoder(r.Body).Decode(&reqBody)
		if err != nil {
			logger.Info("malformed-request", lager.Data{"error": err.Error()})
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		fromVersion := reqBody.From
		if fromVersion == nil {
			latestVersion, found, err := pipelineDB.GetLatestVersionedResource(resourceName)
			if err != nil {
				logger.Info("failed-to-get-latest-versioned-resource", lager.Data{"error": err.Error()})
				w.WriteHeader(http.StatusInternalServerError)
				return
			}

			if found {
				fromVersion = atc.Version(latestVersion.Version)
			}
		}

		scanner := s.scannerFactory.NewResourceScanner(pipelineDB)

		err = scanner.ScanFromVersion(logger, resourceName, fromVersion)
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
