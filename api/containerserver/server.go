package containerserver

import (
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/worker"
	"github.com/pivotal-golang/lager"
)

type Server struct {
	logger lager.Logger

	workerClient worker.Client

	db ContainerDB
}

//go:generate counterfeiter . ContainerDB
type ContainerDB interface {
	GetContainerInfo(handle string) (db.ContainerInfo, bool, error)
	FindContainerInfosByIdentifier(db.ContainerIdentifier) ([]db.ContainerInfo, bool, error)
}

func NewServer(
	logger lager.Logger,
	workerClient worker.Client,
	db ContainerDB,
) *Server {
	return &Server{
		logger:       logger,
		workerClient: workerClient,
		db:           db,
	}
}
