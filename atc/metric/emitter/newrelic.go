package emitter

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/concourse/atc/metric"
	"github.com/pkg/errors"
)

type (
	stats struct {
		created interface{}
		deleted interface{}
	}

	NewRelicEmitter struct {
		Client             *http.Client
		Url                string
		apikey             string
		prefix             string
		checks             *stats
		containers         *stats
		volumes            *stats
		BatchSize          int
		BatchDuration      time.Duration
		DisableCompression bool
		LastEmitTime       time.Time
		NewRelicBatch      []NewRelicEvent
	}

	NewRelicConfig struct {
		AccountID          string        `long:"newrelic-account-id" description:"New Relic Account ID"`
		APIKey             string        `long:"newrelic-api-key" description:"New Relic Insights API Key"`
		Url                string        `long:"newrelic-insights-api-url" default:"https://insights-collector.newrelic.com" description:"Base Url for insights Insert API"`
		ServicePrefix      string        `long:"newrelic-service-prefix" default:"" description:"An optional prefix for emitted New Relic events"`
		BatchSize          uint64        `long:"newrelic-batch-size" default:"2000" description:"Number of events to batch together before emitting"`
		BatchDuration      time.Duration `long:"newrelic-batch-duration" default:"60s" description:"Length of time to wait between emitting until all currently batched events are emitted"`
		DisableCompression bool          `long:"newrelic-batch-disable-compression" description:"Disables compression of the batch before sending it"`
	}

	NewRelicEvent map[string]interface{}
)

func init() {
	metric.Metrics.RegisterEmitter(&NewRelicConfig{})
}

func (config *NewRelicConfig) Description() string { return "NewRelic" }
func (config *NewRelicConfig) IsConfigured() bool {
	return config.AccountID != "" && config.APIKey != ""
}

func (config *NewRelicConfig) NewEmitter() (metric.Emitter, error) {
	client := &http.Client{
		Transport: &http.Transport{ Proxy: http.ProxyFromEnvironment },
		Timeout:   time.Minute,
	}

	return &NewRelicEmitter{
		Client:             client,
		Url:                fmt.Sprintf("%s/v1/accounts/%s/events", config.Url, config.AccountID),
		apikey:             config.APIKey,
		prefix:             config.ServicePrefix,
		containers:         new(stats),
		volumes:            new(stats),
		checks:             new(stats),
		BatchSize:          int(config.BatchSize),
		BatchDuration:      config.BatchDuration,
		DisableCompression: config.DisableCompression,
		LastEmitTime:       time.Now(),
		NewRelicBatch:      make([]NewRelicEvent, 0),
	}, nil
}

func (emitter *NewRelicEmitter) Emit(logger lager.Logger, event metric.Event) {
	logger = logger.Session("new-relic")

	switch event.Name {

	// These are the simple ones that only need a small name transformation
	case "build started",
		"build finished",
		"checks finished",
		"checks started",
		"checks enqueued",
		"checks queue size",
		"worker containers",
		"worker volumes",
		"concurrent requests",
		"concurrent requests limit hit",
		"http response time",
		"database queries",
		"database connections",
		"worker unknown containers",
		"worker unknown volumes":
		emitter.NewRelicBatch = append(emitter.NewRelicBatch, emitter.transformToNewRelicEvent(event, ""))

	// These are periodic metrics that are consolidated and only emitted once
	// per cycle (the emit trigger is chosen because it's currently last in the
	// periodic list, so we should have a coherent view). We do this because
	// new relic has a hard limit on the total number of metrics in a 24h
	// period, so batching similar data where possible makes sense.
	case "checks deleted":
		emitter.checks.deleted = event.Value
	case "containers deleted":
		emitter.containers.deleted = event.Value
	case "containers created":
		emitter.containers.created = event.Value
	case "failed containers":
		singleEvent := emitter.transformToNewRelicEvent(event, "containers")
		singleEvent["failed"] = singleEvent["value"]
		singleEvent["created"] = emitter.containers.created
		singleEvent["deleted"] = emitter.containers.deleted
		delete(singleEvent, "value")
		emitter.NewRelicBatch = append(emitter.NewRelicBatch, singleEvent)

	case "volumes deleted":
		emitter.volumes.deleted = event.Value
	case "volumes created":
		emitter.volumes.created = event.Value
	case "failed volumes":
		singleEvent := emitter.transformToNewRelicEvent(event, "volumes")
		singleEvent["failed"] = singleEvent["value"]
		singleEvent["created"] = emitter.volumes.created
		singleEvent["deleted"] = emitter.volumes.deleted
		delete(singleEvent, "value")
		emitter.NewRelicBatch = append(emitter.NewRelicBatch, singleEvent)

	// And a couple that need a small rename (new relic doesn't like some chars)
	case "scheduling: full duration (ms)":
		emitter.NewRelicBatch = append(emitter.NewRelicBatch, emitter.transformToNewRelicEvent(event,
			"scheduling_full_duration_ms"))
	case "scheduling: loading versions duration (ms)":
		emitter.NewRelicBatch = append(emitter.NewRelicBatch, emitter.transformToNewRelicEvent(event,
			"scheduling_load_duration_ms"))
	case "scheduling: job duration (ms)":
		emitter.NewRelicBatch = append(emitter.NewRelicBatch, emitter.transformToNewRelicEvent(event,
			"scheduling_job_duration_ms"))
	default:
		// Ignore the rest
	}

	duration := time.Since(emitter.LastEmitTime)
	if len(emitter.NewRelicBatch) >= emitter.BatchSize || duration >= emitter.BatchDuration {
		logger.Debug("pre-emit-batch", lager.Data{
			"batch-size":         emitter.BatchSize,
			"current-batch-size": len(emitter.NewRelicBatch),
			"batch-duration":     emitter.BatchDuration,
			"current-duration":   duration,
		})
		emitter.submitBatch(logger)
	}
}

// NewRelic has strict requirements around the structure of the events
// Keys must be alphanumeric and can contain hyphens or underscores
// Values must be sting, int, or unix timestamps. No maps/arrays.
func (emitter *NewRelicEmitter) transformToNewRelicEvent(event metric.Event, nameOverride string) NewRelicEvent {
	name := nameOverride
	if name == "" {
		name = strings.Replace(event.Name, " ", "_", -1)
	}

	eventType := emitter.prefix + name

	payload := NewRelicEvent{
		"eventType": eventType,
		"value":     event.Value,
		"host":      event.Host,
		"timestamp": event.Time.Unix(),
	}

	for k, v := range event.Attributes {
		payload["_"+k] = v
	}
	return payload
}

func (emitter *NewRelicEmitter) submitBatch(logger lager.Logger) {
	batchToSubmit := make([]NewRelicEvent, len(emitter.NewRelicBatch))
	copy(batchToSubmit, emitter.NewRelicBatch)
	emitter.NewRelicBatch = make([]NewRelicEvent, 0)
	emitter.LastEmitTime = time.Now()
	go emitter.emitBatch(logger, batchToSubmit)
}

func (emitter *NewRelicEmitter) emitBatch(logger lager.Logger, payload []NewRelicEvent) {
	batch, err := emitter.marshalJSON(logger, payload)
	if err != nil {
		logger.Error("failed-to-marshal-batch", err)
		return
	}

	req, err := http.NewRequest("POST", emitter.Url, batch)
	if err != nil {
		logger.Error("failed-to-construct-request", err)
		return
	}

	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("X-Insert-Key", emitter.apikey)
	if !emitter.DisableCompression {
		req.Header.Add("Content-Encoding", "gzip")
	}

	resp, err := emitter.Client.Do(req)
	defer resp.Body.Close()
	if err != nil {
		logger.Error("failed-to-send-request",
			errors.Wrap(metric.ErrFailedToEmit, err.Error()))
		return
	}

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		bodyBytes, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			logger.Info("failed-to-read-response-body",
				lager.Data{"error": err.Error(), "status-code": resp.StatusCode})
			return
		}
		logger.Info("received-non-2xx-response-status-code",
			lager.Data{"response-body": string(bodyBytes), "status-code": resp.StatusCode})
		return
	}
}

func (emitter *NewRelicEmitter) marshalJSON(logger lager.Logger, batch []NewRelicEvent) (io.Reader, error) {
	var batchJson *bytes.Buffer
	if emitter.DisableCompression {
		marshaled, err := json.Marshal(batch)
		if err != nil {
			logger.Error("failed-to-serialize-payload", err)
			return nil, err
		}
		batchJson = bytes.NewBuffer(marshaled)
	} else {
		batchJson = bytes.NewBuffer([]byte{})
		encoder := gzip.NewWriter(batchJson)
		defer encoder.Close()
		err := json.NewEncoder(encoder).Encode(batch)
		if err != nil {
			logger.Error("failed-to-compress-and-serialize-payload", err)
			return nil, err
		}
	}
	return batchJson, nil
}
