package webhookserver

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/concourse/atc/db"
)

func (s *Server) TeamWebhook(team db.Team) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		webhookName := r.FormValue(":webhook_name")

		logger := s.logger.Session("webhook", lager.Data{"name": webhookName, "team": team.Name()})

		logger.Info("start")
		defer logger.Info("end")

		token := r.URL.Query().Get("token")

		var payload json.RawMessage
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			logger.Error("failed-to-decode-payload", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		numChecked, err := s.webhooks.CheckResourcesMatchingWebhookPayload(logger, team.ID(), webhookName, payload, token)
		if err != nil {
			if errors.Is(err, db.ErrInvalidWebhookToken) {
				logger.Info("invalid-token")
				w.WriteHeader(http.StatusBadRequest)
				fmt.Fprintf(w, "invalid request: invalid token query parameter")
				return
			}
			if errors.Is(err, db.ErrMissingWebhook) {
				logger.Info("missing-webhook")
				w.WriteHeader(http.StatusNotFound)
				return
			}
			logger.Error("failed-to-check-resources-for-payload", err)
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintf(w, "failed to check resources for payload: %v", err)
			return
		}

		if numChecked > 0 {
			w.WriteHeader(http.StatusCreated)
		} else {
			w.WriteHeader(http.StatusOK)
		}
	})
}
