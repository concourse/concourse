package webhookserver

import (
	"encoding/json"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/concourse/atc"
)

//go:generate counterfeiter . Webhooks

type Webhooks interface {
	CheckResourcesMatchingWebhookPayload(logger lager.Logger, teamID int, name string, payload json.RawMessage, requestToken string) (int, error)
	SaveWebhook(teamID int, webhook atc.Webhook) (bool, error)
}

type Server struct {
	logger   lager.Logger
	webhooks Webhooks
}

func NewServer(
	logger lager.Logger,
	webhooks Webhooks,
) *Server {
	return &Server{
		logger:   logger,
		webhooks: webhooks,
	}
}
