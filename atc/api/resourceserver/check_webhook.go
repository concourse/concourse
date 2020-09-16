package resourceserver

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagerctx"
	"github.com/concourse/concourse/atc/api/present"
	"github.com/concourse/concourse/atc/creds"
	"github.com/concourse/concourse/atc/db"
	"github.com/tedsuo/rata"
)

// CheckResourceWebHook defines a handler for process a check resource request via an access token.
func (s *Server) CheckResourceWebHook(dbPipeline db.Pipeline) http.Handler {
	logger := s.logger.Session("check-resource-webhook")

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resourceName := rata.Param(r, "resource_name")
		webhookToken := r.URL.Query().Get("webhook_token")

		if webhookToken == "" {
			logger.Info("no-webhook-token", lager.Data{"error": "missing webhook_token"})
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		dbResource, found, err := dbPipeline.Resource(resourceName)
		if err != nil {
			logger.Error("database-error", err, lager.Data{"resource-name": resourceName})
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		if !found {
			logger.Info("resource-not-found", lager.Data{"error": fmt.Sprintf("Resource not found %s", resourceName)})
			w.WriteHeader(http.StatusNotFound)
			return
		}

		variables, err := dbPipeline.Variables(logger, s.secretManager, s.varSourcePool)
		if err != nil {
			logger.Error("failed-to-create-var-sources", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		token, err := creds.NewString(variables, dbResource.WebhookToken()).Evaluate()
		if token != webhookToken {
			logger.Info("invalid-token", lager.Data{"error": fmt.Sprintf("invalid token for webhook %s", webhookToken)})
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		dbResourceTypes, err := dbPipeline.ResourceTypes()
		if err != nil {
			logger.Error("failed-to-get-resource-types", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		check, created, err := s.checkFactory.TryCreateCheck(
			lagerctx.NewContext(context.Background(), logger),
			dbResource,
			dbResourceTypes,
			nil,
			true,
		)
		if err != nil {
			s.logger.Error("failed-to-create-check", err)
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(err.Error()))
			return
		}

		if !created {
			s.logger.Info("check-not-created")
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		err = s.checkFactory.NotifyChecker()
		if err != nil {
			s.logger.Error("failed-to-notify-checker", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusCreated)

		err = json.NewEncoder(w).Encode(present.Check(check))
		if err != nil {
			logger.Error("failed-to-encode-check", err)
			w.WriteHeader(http.StatusInternalServerError)
		}
	})
}
