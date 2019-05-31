package resourceserver

import (
	"fmt"
	"net/http"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/concourse/v5/atc"
	"github.com/concourse/concourse/v5/atc/creds"
	"github.com/concourse/concourse/v5/atc/db"
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

		pipelineResource, found, err := dbPipeline.Resource(resourceName)
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

		variables := creds.NewVariables(s.secretManager, dbPipeline.TeamName(), dbPipeline.Name())
		token, err := creds.NewString(variables, pipelineResource.WebhookToken()).Evaluate()
		if token != webhookToken {
			logger.Info("invalid-token", lager.Data{"error": fmt.Sprintf("invalid token for webhook %s", webhookToken)})
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		go func() {
			var fromVersion atc.Version
			resourceConfigID := pipelineResource.ResourceConfigID()
			resourceConfig, found, err := s.resourceConfigFactory.FindResourceConfigByID(resourceConfigID)
			if err != nil {
				logger.Error("failed-to-get-resource-config", err, lager.Data{"resource-config-id": resourceConfigID})
				return
			}

			if found {
				resourceConfigScope, found, err := resourceConfig.FindResourceConfigScopeByID(pipelineResource.ResourceConfigScopeID(), pipelineResource)
				if err != nil {
					logger.Error("failed-to-get-resource-config-scope", err, lager.Data{"resource-config-scope-id": pipelineResource.ResourceConfigScopeID()})
					return
				}

				if found {
					latestVersion, found, err := resourceConfigScope.LatestVersion()
					if err != nil {
						logger.Error("failed-to-get-latest-resource-version", err, lager.Data{"resource-config-id": resourceConfigID})
						return
					}
					if found {
						fromVersion = atc.Version(latestVersion.Version())
					}
				}
			}

			scanner := s.scannerFactory.NewResourceScanner(dbPipeline)
			scanner.ScanFromVersion(logger, pipelineResource.ID(), fromVersion)
		}()

		w.WriteHeader(http.StatusOK)
	})
}
