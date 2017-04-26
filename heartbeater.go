package tsa

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"code.cloudfoundry.org/clock"
	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/lager"
	"github.com/concourse/atc"
	"github.com/concourse/baggageclaim"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/rata"
)

//go:generate counterfeiter . EndpointPicker
type EndpointPicker interface {
	Pick() *rata.RequestGenerator
}

type heartbeater struct {
	logger lager.Logger

	clock       clock.Clock
	interval    time.Duration
	cprInterval time.Duration

	gardenClient       garden.Client
	baggageclaimClient baggageclaim.Client

	atcEndpointPicker EndpointPicker
	tokenGenerator    TokenGenerator

	registration atc.Worker
	clientWriter io.Writer
}

func NewHeartbeater(
	logger lager.Logger,
	clock clock.Clock,
	interval time.Duration,
	cprInterval time.Duration,
	gardenClient garden.Client,
	baggageclaimClient baggageclaim.Client,
	atcEndpointPicker EndpointPicker,
	tokenGenerator TokenGenerator,
	worker atc.Worker,
	clientWriter io.Writer,
) ifrit.Runner {
	return &heartbeater{
		logger: logger,

		clock:       clock,
		interval:    interval,
		cprInterval: cprInterval,

		gardenClient:       gardenClient,
		baggageclaimClient: baggageclaimClient,

		atcEndpointPicker: atcEndpointPicker,
		tokenGenerator:    tokenGenerator,

		registration: worker,
		clientWriter: clientWriter,
	}
}

func (heartbeater *heartbeater) Run(signals <-chan os.Signal, ready chan<- struct{}) error {
	for !heartbeater.register(heartbeater.logger.Session("register")) {
		select {
		case <-heartbeater.clock.NewTimer(time.Second).C():
		case <-signals:
			return nil
		}
	}

	close(ready)

	currentInterval := heartbeater.interval

	for {
		select {
		case <-signals:
			return nil

		case <-heartbeater.clock.NewTimer(currentInterval).C():
			status := heartbeater.heartbeat(heartbeater.logger.Session("heartbeat"))

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

func (heartbeater *heartbeater) register(logger lager.Logger) bool {
	logger.RegisterSink(lager.NewWriterSink(heartbeater.clientWriter, lager.DEBUG))

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

	jwtToken, err := heartbeater.tokenGenerator.GenerateSystemToken()
	if err != nil {
		logger.Error("failed-to-construct-request", err)
		return false
	}

	request.Header.Add("Authorization", "Bearer "+jwtToken)

	request.URL.RawQuery = url.Values{
		"ttl": []string{heartbeater.ttl().String()},
	}.Encode()

	response, err := http.DefaultClient.Do(request)
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

	return true
}

type HeartbeatStatus int

const (
	HeartbeatStatusUnhealthy = iota
	HeartbeatStatusLanded
	HeartbeatStatusGoneAway
	HeartbeatStatusHealthy
)

func (heartbeater *heartbeater) heartbeat(logger lager.Logger) HeartbeatStatus {
	logger.RegisterSink(lager.NewWriterSink(heartbeater.clientWriter, lager.DEBUG))

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

	jwtToken, err := heartbeater.tokenGenerator.GenerateSystemToken()
	if err != nil {
		logger.Error("failed-to-construct-request", err)
		return HeartbeatStatusUnhealthy
	}

	request.Header.Add("Authorization", "Bearer "+jwtToken)

	request.URL.RawQuery = url.Values{
		"ttl": []string{heartbeater.ttl().String()},
	}.Encode()

	response, err := http.DefaultClient.Do(request)
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

func (heartbeater *heartbeater) pingWorker(logger lager.Logger) (atc.Worker, bool) {
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

func (heartbeater *heartbeater) ttl() time.Duration {
	return heartbeater.interval * 2
}
