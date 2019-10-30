package emitter

import (
	"bufio"
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

const (
	newrelicPayloadMaxBytes = 1024 * 1024
)

type (
	stats struct {
		created interface{}
		deleted interface{}
	}

	NewRelicEmitter struct {
		prefix       string
		containers   *stats
		volumes      *stats
		batchEmitter *batchEmitter
	}

	NewRelicConfig struct {
		AccountID         string        `long:"newrelic-account-id" description:"New Relic Account ID"`
		APIKey            string        `long:"newrelic-api-key" description:"New Relic Insights API Key"`
		ServicePrefix     string        `long:"newrelic-service-prefix" default:"" description:"An optional prefix for emitted New Relic events"`
		EnableCompression bool          `long:"newrelic-metric-compression" description:"New Relic payload compression flag"`
		FlushInterval     time.Duration `long:"newrelic-flush-interval" default:"60s" description:"Newrelic metric flush interval in seconds"`
	}

	singlePayload map[string]interface{}

	batchPayload []byte

	batchEmitter struct {
		client        *http.Client
		url           string
		apikey        string
		emitBuffer    []byte
		lastEmitTime  time.Time
		compression   bool
		flushInterval time.Duration
	}

	loggableError struct {
		Action string
		Error  error
	}
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
		prefix:     config.ServicePrefix,
		containers: new(stats),
		volumes:    new(stats),
		batchEmitter: &batchEmitter{
			client:        client,
			url:           fmt.Sprintf("https://insights-collector.newrelic.com/v1/accounts/%s/events", config.AccountID),
			apikey:        config.APIKey,
			compression:   config.EnableCompression,
			flushInterval: config.FlushInterval,
			lastEmitTime:  time.Now(),
		},
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

func (emitter *batchEmitter) emitBatch(logger lager.Logger, payload batchPayload) {
	var (
		payloadReader io.Reader
		err           error
	)

	if emitter.compression {
		payloadReader, err = gZipBuffer(payload)
		if err != nil {
			logger.Error("failed-to-zip-payload", errors.Wrap(metric.ErrFailedToEmit, err.Error()))
			return
		}
	} else {
		payloadReader = bytes.NewBuffer(payload)
	}

	req, err := http.NewRequest("POST", emitter.url, payloadReader)
	if err != nil {
		logger.Error("failed-to-construct-request", err)
		return
	}

	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("X-Insert-Key", emitter.apikey)
	if emitter.compression {
		req.Header.Add("Content-Encoding", "gzip")
	}

	resp, err := emitter.client.Do(req)
	if err != nil {
		logger.Error("failed-to-send-request",
			errors.Wrap(metric.ErrFailedToEmit, err.Error()))
		return
	}
	resp.Body.Close()
}

func (emitter *NewRelicEmitter) Emit(logger lager.Logger, event metric.Event) {
	switch event.Name {
	// These are the simple ones that only need a small name transformation
	case "build started",
		"build finished",
		"worker containers",
		"worker volumes",
		"http response time",
		"database queries",
		"database connections",
		"worker unknown containers",
		"worker unknown volumes":
		emitter.batchEmitter.Emit(logger, emitter.simplePayload(logger, event, ""))

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
		emitter.batchEmitter.Emit(logger, newPayload)

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
		emitter.batchEmitter.Emit(logger, newPayload)

	// And a couple that need a small rename (new relic doesn't like some chars)
	case "scheduling: full duration (ms)":
		emitter.batchEmitter.Emit(logger, emitter.simplePayload(logger, event, "scheduling_full_duration_ms"))
	case "scheduling: loading versions duration (ms)":
		emitter.batchEmitter.Emit(logger, emitter.simplePayload(logger, event, "scheduling_load_duration_ms"))
	case "scheduling: job duration (ms)":
		emitter.batchEmitter.Emit(logger, emitter.simplePayload(logger, event, "scheduling_job_duration_ms"))

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
		emitter.batchEmitter.Emit(logger, singlePayload)

	}
}

func (emitter *NewRelicEmitter) BufferPayloadSize() int {
	size := 0
	if emitter.batchEmitter.emitBuffer != nil {
		size = len(emitter.batchEmitter.emitBuffer)
	}
	return size
}

func (emitter *NewRelicEmitter) SetUrl(url string) {
	emitter.batchEmitter.url = url
}

func (emitter *batchEmitter) Flush(logger lager.Logger) {
	if !emitter.flushThreshold() {
		return
	}

	batchPayload := emitter.flush()
	if batchPayload != nil && len(batchPayload) > 0 {
		emitter.updateLastEmitTime()
		go emitter.emitBatch(logger, batchPayload)
	}
}

func (emitter *batchEmitter) Emit(logger lager.Logger, payload singlePayload) {
	// enqueue the data
	// check the time since last time we emitted the data, if it is great than the threshold, emit right now
	payloadToSend, err := emitter.enqueue(payload)
	if err != nil {
		logger.Error(err.Action, err.Error)
		return
	}
	if payloadToSend != nil {
		emitter.updateLastEmitTime()
		go emitter.emitBatch(logger, payloadToSend)
	}

	emitter.Flush(logger)
}

// enqueue new payload
// if enqueue the new payload would exceed the max limit (1Mb)
// return everything that's enqueued and enqueue the new payload
func (emitter *batchEmitter) enqueue(newPayload singlePayload) (batchPayload, *loggableError) {
	var (
		tempBuff []byte
		buff     batchPayload
	)
	newPayloadData, err := json.Marshal(newPayload)
	if err != nil {
		return nil, newLoggableError("failed-to-serialize-new-payload", err)
	}

	if newPayloadData == nil || len(newPayloadData) == 0 {
		return nil, newLoggableError("empty-new-payload", err)
	}

	if len(emitter.emitBuffer) != 0 {
		tempBuff = append(emitter.emitBuffer, ',')
	}
	tempBuff = append(tempBuff, newPayloadData...)

	payloadSize, sizeError := emitter.bufferSize(tempBuff)
	if sizeError != nil {
		return nil, sizeError
	}

	// when combined new payload exceeds 1MB
	if payloadSize > newrelicPayloadMaxBytes-2 { // reduce '[' and ']' for the json payload
		buff = formatBatchEmitBuffer(emitter.emitBuffer)
		emitter.emitBuffer = newPayloadData
	} else {
		emitter.emitBuffer = tempBuff
	}
	return buff, nil
}

func (emitter *batchEmitter) bufferSize(buff []byte) (int, *loggableError) {
	payloadSize := len(buff)
	// for better performance, we only check compressed data size when the data in the buffer is larger than 1MB
	if emitter.compression && payloadSize >= newrelicPayloadMaxBytes-2 { // reduce size for '[' and ']'
		zippedBuffer, err := gZipBuffer(buff)
		if err != nil {
			return 0, newLoggableError("failed-to-gzip-payload", err)
		}

		zippedData, err := ioutil.ReadAll(zippedBuffer)
		if err != nil {
			return 0, newLoggableError("failed-to-use-gzip-buffer-payload", err)
		}
		payloadSize = len(zippedData)
	}
	return payloadSize, nil
}

func (emitter *batchEmitter) flush() batchPayload {
	var buff batchPayload

	if emitter.emitBuffer != nil {
		buff = formatBatchEmitBuffer(emitter.emitBuffer)
		emitter.emitBuffer = nil
	}
	return buff
}

func (emitter *batchEmitter) updateLastEmitTime() {
	emitter.lastEmitTime = time.Now()
}

func (emitter *batchEmitter) flushThreshold() bool {
	return time.Now().Sub(emitter.lastEmitTime) > emitter.flushInterval
}

func gZipBuffer(body []byte) (reader io.Reader, err error) {
	readBuffer := bufio.NewReader(bytes.NewReader(body))
	buffer := bytes.NewBuffer([]byte{})
	writer := gzip.NewWriter(buffer)

	defer func() {
		cerr := writer.Close()
		if err == nil {
			err = cerr
		}
	}()

	_, err = readBuffer.WriteTo(writer)
	if err != nil {
		return nil, err
	}
	return buffer, nil
}

func newLoggableError(action string, err error) *loggableError {
	return &loggableError{Action: action, Error: err}
}

func formatBatchEmitBuffer(buffer []byte) batchPayload {
	buffTemp := buffer
	buffTemp = append(append([]byte{'['}, buffTemp...), ']')
	returnBuff := make([]byte, len(buffTemp))
	copy(returnBuff, buffTemp)
	return returnBuff
}
