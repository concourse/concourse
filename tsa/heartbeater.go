package tsa

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"time"

	"code.cloudfoundry.org/clock"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagerctx"
	"github.com/concourse/baggageclaim"
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/worker/gclient"
	"github.com/tedsuo/rata"
)

//go:generate counterfeiter . EndpointPicker
type EndpointPicker interface {
	Pick() *rata.RequestGenerator
}

type Heartbeater struct {
	clock       clock.Clock
	interval    time.Duration
	cprInterval time.Duration

	gardenClient       gclient.Client
	baggageclaimClient baggageclaim.Client

	atcEndpointPicker EndpointPicker
	httpClient        *http.Client

	registration atc.Worker
	eventWriter  EventWriter
}

func NewHeartbeater(
	clock clock.Clock,
	interval time.Duration,
	cprInterval time.Duration,
	gardenClient gclient.Client,
	baggageclaimClient baggageclaim.Client,
	atcEndpointPicker EndpointPicker,
	httpClient *http.Client,
	worker atc.Worker,
	eventWriter EventWriter,
) *Heartbeater {
	return &Heartbeater{
		clock:       clock,
		interval:    interval,
		cprInterval: cprInterval,

		gardenClient:       gardenClient,
		baggageclaimClient: baggageclaimClient,

		atcEndpointPicker: atcEndpointPicker,
		httpClient:        httpClient,

		registration: worker,
		eventWriter:  eventWriter,
	}
}

func (heartbeater *Heartbeater) Heartbeat(ctx context.Context) error {
	logger := lagerctx.FromContext(ctx)

	logger.Info("start")
	defer logger.Info("done")

	for !heartbeater.register(logger.Session("register")) {
		select {
		case <-heartbeater.clock.NewTimer(time.Second).C():
		case <-ctx.Done():
			return nil
		}
	}

	currentInterval := heartbeater.interval

	for {
		select {
		case <-ctx.Done():
			return nil

		case <-heartbeater.clock.NewTimer(currentInterval).C():
			status := heartbeater.heartbeat(logger.Session("heartbeat"))
			switch status {
			case HeartbeatStatusGoneAway:
				return nil
			case HeartbeatStatusLanded:
				return nil
			case HeartbeatStatusHealthy:
				currentInterval = heartbeater.interval
			default:
				currentInterval = heartbeater.cprInterval
			}
		}
	}

}

func (heartbeater *Heartbeater) register(logger lager.Logger) bool {
	heartbeatData := lager.Data{
		"worker-platform": heartbeater.registration.Platform,
		"worker-address":  heartbeater.registration.GardenAddr,
		"worker-tags":     strings.Join(heartbeater.registration.Tags, ","),
	}

	logger.Info("start", heartbeatData)
	defer logger.Info("done", heartbeatData)

	registration, ok := heartbeater.pingWorker(logger)
	if !ok {
		return false
	}

	payload, err := json.Marshal(registration)
	if err != nil {
		logger.Error("failed-to-marshal-registration", err)
		return false
	}

	request, err := heartbeater.atcEndpointPicker.Pick().CreateRequest(atc.RegisterWorker, nil, bytes.NewBuffer(payload))
	if err != nil {
		logger.Error("failed-to-construct-request", err)
		return false
	}

	request.URL.RawQuery = url.Values{
		"ttl": []string{heartbeater.ttl().String()},
	}.Encode()

	response, err := heartbeater.httpClient.Do(request)
	if err != nil {
		logger.Error("failed-to-register", err)
		return false
	}

	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		logger.Error("bad-response", nil, lager.Data{
			"status-code": response.StatusCode,
		})

		return false
	}

	err = heartbeater.eventWriter.Registered()
	if err != nil {
		logger.Error("failed-to-emit-registered-event", err)
		return true
	}

	return true
}

type HeartbeatStatus int

const (
	HeartbeatStatusUnhealthy = iota
	HeartbeatStatusLanded
	HeartbeatStatusGoneAway
	HeartbeatStatusHealthy
)

func (heartbeater *Heartbeater) heartbeat(logger lager.Logger) HeartbeatStatus {
	heartbeatData := lager.Data{
		"worker-platform": heartbeater.registration.Platform,
		"worker-address":  heartbeater.registration.GardenAddr,
		"worker-tags":     strings.Join(heartbeater.registration.Tags, ","),
	}

	logger.Info("start", heartbeatData)
	defer logger.Info("done", heartbeatData)

	registration, ok := heartbeater.pingWorker(logger)
	if !ok {
		return HeartbeatStatusUnhealthy
	}

	payload, err := json.Marshal(registration)
	if err != nil {
		logger.Error("failed-to-marshal-registration", err)
		return HeartbeatStatusUnhealthy
	}

	request, err := heartbeater.atcEndpointPicker.Pick().CreateRequest(atc.HeartbeatWorker, rata.Params{
		"worker_name": heartbeater.registration.Name,
	}, bytes.NewBuffer(payload))
	if err != nil {
		logger.Error("failed-to-construct-request", err)
		return HeartbeatStatusUnhealthy
	}

	request.URL.RawQuery = url.Values{
		"ttl": []string{heartbeater.ttl().String()},
	}.Encode()

	response, err := heartbeater.httpClient.Do(request)
	if err != nil {
		logger.Error("failed-to-heartbeat", err)
		return HeartbeatStatusUnhealthy
	}

	defer response.Body.Close()

	if response.StatusCode == http.StatusNotFound {
		logger.Debug("worker-has-gone-away")
		return HeartbeatStatusGoneAway
	}

	if response.StatusCode != http.StatusOK {
		logger.Error("bad-response", nil, lager.Data{
			"status-code": response.StatusCode,
		})

		return HeartbeatStatusUnhealthy
	}

	err = heartbeater.eventWriter.Heartbeated()
	if err != nil {
		logger.Error("failed-to-emit-heartbeated-event", err)
	}

	var workerInfo atc.Worker
	err = json.NewDecoder(response.Body).Decode(&workerInfo)
	if err != nil {
		logger.Error("failed-to-decode-response", err)
		return HeartbeatStatusUnhealthy
	}

	if workerInfo.State == "landed" {
		logger.Debug("worker-landed")
		return HeartbeatStatusLanded
	}

	return HeartbeatStatusHealthy
}

func (heartbeater *Heartbeater) pingWorker(logger lager.Logger) (atc.Worker, bool) {
	registration := heartbeater.registration

	beforeGarden := time.Now()

	healthy := true

	containers, err := heartbeater.gardenClient.Containers(nil)
	if err != nil {
		logger.Error("failed-to-fetch-containers", err)
		healthy = false
	}

	afterGarden := time.Now()

	beforeBaggageclaim := time.Now()

	volumes, err := heartbeater.baggageclaimClient.ListVolumes(logger.Session("list-volumes"), nil)
	if err != nil {
		logger.Error("failed-to-list-volumes", err)
		healthy = false
	}

	afterBaggageclaim := time.Now()

	durationData := lager.Data{
		"garden-took":       afterGarden.Sub(beforeGarden).String(),
		"baggageclaim-took": afterBaggageclaim.Sub(beforeBaggageclaim).String(),
	}

	if healthy {
		logger.Debug("reached-worker", durationData)
	} else {
		logger.Info("failed-to-reach-worker", durationData)
		return atc.Worker{}, false
	}

	registration.ActiveContainers = len(containers)
	registration.ActiveVolumes = len(volumes)

	return registration, true
}

func (heartbeater *Heartbeater) ttl() time.Duration {
	return heartbeater.interval * 2
}
