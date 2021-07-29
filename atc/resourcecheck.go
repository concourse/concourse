package atc

import "time"

var (
	DefaultCheckInterval   time.Duration
	DefaultWebhookInterval time.Duration
)

type CheckRequestBody struct {
	From    Version `json:"from"`
	Shallow bool    `json:"shallow"`
}
