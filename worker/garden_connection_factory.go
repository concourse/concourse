package worker

import (
	"net"

	gconn "github.com/cloudfoundry-incubator/garden/client/connection"
	"github.com/concourse/atc/db"
	"github.com/pivotal-golang/lager"
)

//go:generate counterfeiter . GardenConnectionFactoryDB
type GardenConnectionFactoryDB interface {
	GetWorker(int) (db.SavedWorker, bool, error)
}

//go:generate counterfeiter . GardenConnectionFactory
type GardenConnectionFactory interface {
	BuildConnection() gconn.Connection
	BuildConnectionFromDB() (gconn.Connection, error)
}

type gardenConnectionFactory struct {
	db       GardenConnectionFactoryDB
	dialer   gconn.DialerFunc
	logger   lager.Logger
	workerID int
	address  string
}

func NewGardenConnectionFactory(
	db GardenConnectionFactoryDB,
	dialer gconn.DialerFunc,
	logger lager.Logger,
	workerID int,
	address string,
) GardenConnectionFactory {
	return &gardenConnectionFactory{
		db:       db,
		dialer:   dialer,
		logger:   logger,
		workerID: workerID,
		address:  address,
	}
}

func (gcf *gardenConnectionFactory) BuildConnection() gconn.Connection {
	return gcf.buildConnection(gcf.address)
}

func (gcf *gardenConnectionFactory) BuildConnectionFromDB() (gconn.Connection, error) {
	savedWorker, found, err := gcf.db.GetWorker(gcf.workerID)
	if err != nil {
		return nil, err
	}

	if !found {
		return nil, ErrMissingWorker
	}

	return gcf.buildConnection(savedWorker.GardenAddr), nil
}

func (gcf *gardenConnectionFactory) buildConnection(address string) gconn.Connection {
	var connection gconn.Connection

	if gcf.dialer == nil {
		connection = gconn.NewWithLogger("tcp", address, gcf.logger)
	} else {
		dialer := func(string, string) (net.Conn, error) {
			return gcf.dialer("tcp", address)
		}
		connection = gconn.NewWithDialerAndLogger(dialer, gcf.logger)
	}

	return connection
}
