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

type (
	stats struct {
		created interface{}
		deleted interface{}
	}

	NewRelicEmitter struct {
		client     *http.Client
		url        string
		apikey     string
		prefix     string
		containers *stats
		volumes    *stats
	}

	NewRelicConfig struct {
		AccountID     string `long:"newrelic-account-id" description:"New Relic Account ID"`
		APIKey        string `long:"newrelic-api-key" description:"New Relic Insights API Key"`
		ServicePrefix string `long:"newrelic-service-prefix" default:"" description:"An optional prefix for emitted New Relic events"`
	}

	singlePayload map[string]interface{}
	fullPayload   []singlePayload
)

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
		client:     client,
		url:        fmt.Sprintf("https://insights-collector.newrelic.com/v1/accounts/%s/events", config.AccountID),
		apikey:     config.APIKey,
		prefix:     config.ServicePrefix,
		containers: new(stats),
		volumes:    new(stats),
	}, nil
}

func (emitter *NewRelicEmitter) simplePayload(logger lager.Logger, event metric.Event, nameOverride string) singlePayload {
	name := nameOverride
	if name == "" {
		name = strings.Replace(event.Name, " ", "_", -1)
	}

	eventType := fmt.Sprintf("%s%s", emitter.prefix, name)

	payload := singlePayload{
		"eventType": eventType,
		"value":     event.Value,
		"state":     string(event.State),
		"host":      event.Host,
		"timestamp": event.Time.Unix(),
	}

	for k, v := range event.Attributes {
		payload[fmt.Sprintf("_%s", k)] = v
	}
	return payload
}

func (emitter *NewRelicEmitter) emitPayload(logger lager.Logger, payload fullPayload) {
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

func (emitter *NewRelicEmitter) Emit(logger lager.Logger, event metric.Event) {
	payload := make(fullPayload, 0)

	switch event.Name {

	// These are the simple ones that only need a small name transformation
	case "build started",
		"build finished",
		"worker containers",
		"worker volumes",
		"http response time",
		"database queries",
		"database connections":
		payload = append(payload, emitter.simplePayload(logger, event, ""))

	// These are periodic metrics that are consolidated and only emitted once
	// per cycle (the emit trigger is chosen because it's currently last in the
	// periodic list, so we should have a coherent view). We do this because
	// new relic has a hard limit on the total number of metrics in a 24h
	// period, so batching similar data where possible makes sense.
	case "containers deleted":
		emitter.containers.deleted = event.Value
	case "containers created":
		emitter.containers.created = event.Value
	case "failed containers":
		newPayload := emitter.simplePayload(logger, event, "containers")
		newPayload["failed"] = newPayload["value"]
		newPayload["created"] = emitter.containers.created
		newPayload["deleted"] = emitter.containers.deleted
		delete(newPayload, "value")
		payload = append(payload, newPayload)

	case "volumes deleted":
		emitter.volumes.deleted = event.Value
	case "volumes created":
		emitter.volumes.created = event.Value
	case "failed volumes":
		newPayload := emitter.simplePayload(logger, event, "volumes")
		newPayload["failed"] = newPayload["value"]
		newPayload["created"] = emitter.volumes.created
		newPayload["deleted"] = emitter.volumes.deleted
		delete(newPayload, "value")
		payload = append(payload, newPayload)

	// And a couple that need a small rename (new relic doesn't like some chars)
	case "scheduling: full duration (ms)":
		payload = append(payload, emitter.simplePayload(logger, event, "scheduling_full_duration_ms"))
	case "scheduling: loading versions duration (ms)":
		payload = append(payload, emitter.simplePayload(logger, event, "scheduling_load_duration_ms"))
	case "scheduling: job duration (ms)":
		payload = append(payload, emitter.simplePayload(logger, event, "scheduling_job_duration_ms"))
	default:
		// Ignore the rest
	}

	// But also log any metric that's not EventStateOK, even if we're not
	// otherwise recording it. (This won't be easily graphable, that's okay,
	// this is more for monitoring synthetics)
	if event.State != metric.EventStateOK {
		singlePayload := emitter.simplePayload(logger, event, "alert")
		// We don't have friendly names for all the metrics, and part of the
		// point of this alert is to catch events we should be logging but
		// didn't; therefore, be consistently inconsistent and use the
		// concourse metric names, not our translation layer.
		singlePayload["metric"] = event.Name
		payload = append(payload, singlePayload)
	}

	if len(payload) > 0 {
		emitter.emitPayload(logger, payload)
	}
}
