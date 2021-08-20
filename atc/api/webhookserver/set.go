package webhookserver

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db"
)

func (s *Server) SetTeamWebhook(team db.Team) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		logger := s.logger.Session("set-webhook")

		var webhook atc.Webhook
		if err := json.NewDecoder(r.Body).Decode(&webhook); err != nil {
			logger.Error("malformed-request", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		if err := webhook.Validate(); err != nil {
			logger.Error("invalid-webhook", err)
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprintf(w, "invalid webhook: %v", err)
			return
		}

		created, err := s.webhooks.SaveWebhook(team.ID(), webhook)
		if err != nil {
			logger.Error("failed-to-save-webhook", err)
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintf(w, "failed to save config: %v", err)
			return
		}

		if created {
			w.WriteHeader(http.StatusCreated)
		} else {
			w.WriteHeader(http.StatusNoContent)
		}
	})
}
