package metric

import (
	"errors"
	"time"

	"code.cloudfoundry.org/lager"
	"github.com/bigdatadev/goryman"
)

type eventEmission struct {
	event  goryman.Event
	logger lager.Logger
}

var riemannClient *goryman.GorymanClient
var eventHost string
var eventTags []string
var eventAttributes map[string]string

var clientConnected bool
var emissions = make(chan eventEmission, 1000)

var errQueueFull = errors.New("event queue is full")

func Initialize(logger lager.Logger, riemannAddr string, host string, tags []string, attributes map[string]string) {
	client := goryman.NewGorymanClient(riemannAddr)

	riemannClient = client
	eventHost = host
	eventTags = tags
	eventAttributes = attributes

	go emitLoop()
	go periodicallyEmit(logger.Session("periodic"), 10*time.Second)
}

func emit(logger lager.Logger, event goryman.Event) {
	logger.Debug("emit")

	if riemannClient == nil {
		return
	}

	event.Host = eventHost
	event.Time = time.Now().Unix()
	event.Tags = append(event.Tags, eventTags...)

	mergedAttributes := map[string]string{}
	for k, v := range eventAttributes {
		mergedAttributes[k] = v
	}

	if event.Attributes != nil {
		for k, v := range event.Attributes {
			mergedAttributes[k] = v
		}
	}

	event.Attributes = mergedAttributes

	select {
	case emissions <- eventEmission{logger: logger, event: event}:
	default:
		logger.Error("queue-full", nil)
	}
}

func emitLoop() {
	for emission := range emissions {
		if !clientConnected {
			err := riemannClient.Connect()
			if err != nil {
				emission.logger.Error("connection-failed", err)
				continue
			}

			clientConnected = true
		}

		err := riemannClient.SendEvent(&emission.event)
		if err != nil {
			emission.logger.Error("failed-to-emit", err)

			if err := riemannClient.Close(); err != nil {
				emission.logger.Error("failed-to-close", err)
			}

			clientConnected = false
		}
	}
}
