package emitter

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/atc/metric"
)

type NewRelicEmitter struct {
	client *http.Client
	url    string
	apikey string
	prefix string
}

type NewRelicConfig struct {
	AccountID     string `long:"newrelic-account-id" description:"New Relic Account ID"`
	APIKey        string `long:"newrelic-api-key" description:"New Relic Insights API Key"`
	ServicePrefix string `long:"newrelic-service-prefix" default:"" description:"An optional prefix for emitted New Relic events"`
}

func init() {
	metric.RegisterEmitter(&NewRelicConfig{})
}

func (config *NewRelicConfig) Description() string { return "NewRelic" }
func (config *NewRelicConfig) IsConfigured() bool {
	return config.AccountID != "" && config.APIKey != ""
}

func (config *NewRelicConfig) NewEmitter() (metric.Emitter, error) {
	client := &http.Client{
		Transport: &http.Transport{},
		Timeout:   time.Minute,
	}

	return &NewRelicEmitter{
		client: client,
		url:    fmt.Sprintf("https://insights-collector.newrelic.com/v1/accounts/%s/events", config.AccountID),
		apikey: config.APIKey,
		prefix: config.ServicePrefix,
	}, nil
}

func (emitter *NewRelicEmitter) Emit(logger lager.Logger, event metric.Event) {

	payload := [1]map[string]interface{}{{
		"eventType": fmt.Sprintf("%s%s", emitter.prefix, strings.Replace(event.Name, " ", "_", -1)),
		"value":     event.Value,
		"state":     string(event.State),
		"host":      event.Host,
		"timestamp": event.Time.Unix(),
	}}

	for k, v := range event.Attributes {
		payload[0][fmt.Sprintf("_%s", k)] = v
	}

	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		logger.Error("failed-to-serialize-payload", err)
		return
	}

	req, err := http.NewRequest("POST", emitter.url, bytes.NewBuffer(payloadJSON))
	if err != nil {
		logger.Error("failed-to-construct-request", err)
	}

	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("X-Insert-Key", emitter.apikey)

	resp, err := emitter.client.Do(req)

	if err != nil {
		logger.Error("failed-to-send-request", err)
		return
	}

	resp.Body.Close()
}
