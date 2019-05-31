package resourceserver

import (
	"encoding/json"
	"net/http"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/concourse/v5/atc"
	"github.com/concourse/concourse/v5/atc/db"
	"github.com/tedsuo/rata"
)

func (s *Server) CheckResourceType(dbPipeline db.Pipeline) http.Handler {
	logger := s.logger.Session("check-resource-type")

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resourceName := rata.Param(r, "resource_type_name")

		var reqBody atc.CheckRequestBody
		err := json.NewDecoder(r.Body).Decode(&reqBody)
		if err != nil {
			logger.Info("malformed-request", lager.Data{"error": err.Error()})
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		dbResourceType, found, err := dbPipeline.ResourceType(resourceName)
		if err != nil {
			logger.Error("failed-to-get-resource-type", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		if !found {
			logger.Debug("resource-type-not-found", lager.Data{"resource": resourceName})
			w.WriteHeader(http.StatusNotFound)
			return
		}

		scanner := s.scannerFactory.NewResourceTypeScanner(dbPipeline)

		err = scanner.ScanFromVersion(logger, dbResourceType.ID(), reqBody.From)
		switch err.(type) {
		case db.ResourceTypeNotFoundError:
			w.WriteHeader(http.StatusNotFound)
		case error:
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(err.Error()))
		default:
			w.WriteHeader(http.StatusOK)
		}
	})
}
