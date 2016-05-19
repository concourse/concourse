package worker

import (
	gconn "github.com/cloudfoundry-incubator/garden/client/connection"
	"github.com/concourse/atc/worker/transport"
	"github.com/pivotal-golang/lager"
)

//go:generate counterfeiter . GardenConnectionFactory
type GardenConnectionFactory interface {
	BuildConnection() gconn.Connection
}

type gardenConnectionFactory struct {
	db         transport.TransportDB
	logger     lager.Logger
	workerName string
	address    string
}

func NewGardenConnectionFactory(
	db transport.TransportDB,
	logger lager.Logger,
	workerName string,
) GardenConnectionFactory {
	return &gardenConnectionFactory{
		db:         db,
		logger:     logger,
		workerName: workerName,
	}
}

func (gcf *gardenConnectionFactory) BuildConnection() gconn.Connection {
	// the request generator's address doesn't matter because it's overwritten by the worker lookup clients
	hijackStreamer := transport.NewHijackStreamer(gcf.logger, gcf.workerName, gcf.db)
	return gconn.NewWithHijacker(hijackStreamer, gcf.logger)
}
