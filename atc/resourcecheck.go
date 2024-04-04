package atc

import "time"

var (
	DefaultCheckInterval   time.Duration
	DefaultWebhookInterval time.Duration
	DefaultResourceTypeInterval time.Duration
)

type CheckRequestBody struct {
	From    Version `json:"from"`
	Shallow bool    `json:"shallow"`
}
