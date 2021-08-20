package webhookserver

import (
	"errors"
	"net/http"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/concourse/atc/db"
)

func (s *Server) DestroyTeamWebhook(team db.Team) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		webhookName := r.FormValue(":webhook_name")

		logger := s.logger.Session("destroy-webhook", lager.Data{"name": webhookName, "team": team.Name()})
		logger.Debug("destroying-webhook")

		err := s.webhooks.DeleteWebhook(team.ID(), webhookName)
		if err != nil {
			if errors.Is(err, db.ErrMissingWebhook) {
				logger.Info("missing-webhook")
				w.WriteHeader(http.StatusNotFound)
				return
			}
			logger.Error("failed-to-delete-webhook", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusNoContent)
	})
}
