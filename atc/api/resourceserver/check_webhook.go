package resourceserver

import (
	"context"
	"net/http"

	"code.cloudfoundry.org/lager/v3"
	"code.cloudfoundry.org/lager/v3/lagerctx"
	"github.com/bytedance/sonic"
	"github.com/concourse/concourse/atc/api/present"
	"github.com/concourse/concourse/atc/creds"
	"github.com/concourse/concourse/atc/db"
	"github.com/tedsuo/rata"
)

// CheckResourceWebHook defines a handler for process a check resource request via an access token.
func (s *Server) CheckResourceWebHook(dbPipeline db.Pipeline) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resourceName := rata.Param(r, "resource_name")
		webhookToken := r.URL.Query().Get("webhook_token")

		logger := s.logger.Session("check-resource-webhook", lager.Data{
			"resource": resourceName,
		})

		if webhookToken == "" {
			logger.Info("no-webhook-token", lager.Data{"error": "missing webhook_token"})
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		dbResource, found, err := dbPipeline.Resource(resourceName)
		if err != nil {
			logger.Error("failed-to-get-resource", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		if !found {
			logger.Info("resource-not-found")
			w.WriteHeader(http.StatusNotFound)
			return
		}

		secretsParams := creds.SecretLookupParams{
			Team:         dbPipeline.TeamName(),
			Pipeline:     dbPipeline.Name(),
			InstanceVars: dbPipeline.InstanceVars(),
		}

		variables, err := dbPipeline.Variables(logger, s.secretManager, s.varSourcePool, secretsParams)
		if err != nil {
			logger.Error("failed-to-create-var-sources", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		token, err := creds.NewString(variables, dbResource.WebhookToken()).Evaluate()
		if err != nil {
			logger.Error("failed-to-evaluate-webhook-token", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		if token != webhookToken {
			logger.Info("invalid-token", lager.Data{"token": webhookToken})
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		dbResourceTypes, err := dbPipeline.ResourceTypes()
		if err != nil {
			logger.Error("failed-to-get-resource-types", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		build, created, err := s.checkFactory.TryCreateCheck(
			lagerctx.NewContext(context.Background(), logger),
			dbResource,
			dbResourceTypes,
			nil,
			true,
			false,
			true,
		)
		if err != nil {
			logger.Error("failed-to-create-check", err)
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(err.Error()))
			return
		}

		if !created {
			logger.Info("check-not-created")
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusCreated)

		err = sonic.ConfigDefault.NewEncoder(w).Encode(present.Build(build, nil, nil))
		if err != nil {
			logger.Error("failed-to-encode-check", err)
			w.WriteHeader(http.StatusInternalServerError)
		}
	})
}
