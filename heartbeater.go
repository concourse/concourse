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

	"code.cloudfoundry.org/garden"
	"github.com/concourse/atc"
	"code.cloudfoundry.org/clock"
	"code.cloudfoundry.org/lager"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/rata"
)

type heartbeater struct {
	logger lager.Logger

	clock       clock.Clock
	interval    time.Duration
	cprInterval time.Duration

	gardenClient   garden.Client
	atcEndpoint    *rata.RequestGenerator
	tokenGenerator TokenGenerator

	registration atc.Worker
	clientWriter io.Writer
}

func NewHeartbeater(
	logger lager.Logger,
	clock clock.Clock,
	interval time.Duration,
	cprInterval time.Duration,
	gardenClient garden.Client,
	atcEndpoint *rata.RequestGenerator,
	tokenGenerator TokenGenerator,
	worker atc.Worker,
	clientWriter io.Writer,
) ifrit.Runner {
	return &heartbeater{
		logger: logger,

		clock:       clock,
		interval:    interval,
		cprInterval: cprInterval,

		gardenClient:   gardenClient,
		atcEndpoint:    atcEndpoint,
		tokenGenerator: tokenGenerator,

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
			healthy := heartbeater.register(heartbeater.logger.Session("heartbeat"))
			if healthy {
				currentInterval = heartbeater.interval
			} else {
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

	before := time.Now()

	containers, err := heartbeater.gardenClient.Containers(nil)
	if err != nil {
		logger.Error("failed-to-fetch-containers", err)
		return false
	}

	after := time.Now()

	logger.Debug("reached-worker", lager.Data{"took": after.Sub(before).String()})

	heartbeater.registration.ActiveContainers = len(containers)

	payload, err := json.Marshal(heartbeater.registration)
	if err != nil {
		logger.Error("failed-to-marshal-registration", err)
		return false
	}

	request, err := heartbeater.atcEndpoint.CreateRequest(atc.RegisterWorker, nil, bytes.NewBuffer(payload))
	if err != nil {
		logger.Error("failed-to-construct-request", err)
		return false
	}

	jwtToken, err := heartbeater.tokenGenerator.GenerateToken()
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

func (heartbeater *heartbeater) ttl() time.Duration {
	return heartbeater.interval * 2
}
