package containerserver

import (
	"time"

	"code.cloudfoundry.org/clock"
	"code.cloudfoundry.org/lager"
	"github.com/concourse/concourse/atc/creds"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/gc"
	"github.com/concourse/concourse/atc/runtime"
)

type Pool interface {
	LocateContainer(logger lager.Logger, teamID int, handle string) (runtime.Container, runtime.Worker, bool, error)
}

type Server struct {
	logger lager.Logger

	workerPool              Pool
	secretManager           creds.Secrets
	varSourcePool           creds.VarSourcePool
	interceptTimeoutFactory InterceptTimeoutFactory
	interceptUpdateInterval time.Duration
	containerRepository     db.ContainerRepository
	destroyer               gc.Destroyer
	clock                   clock.Clock
}

func NewServer(
	logger lager.Logger,
	workerPool Pool,
	secretManager creds.Secrets,
	varSourcePool creds.VarSourcePool,
	interceptTimeoutFactory InterceptTimeoutFactory,
	interceptUpdateInterval time.Duration,
	containerRepository db.ContainerRepository,
	destroyer gc.Destroyer,
	clock clock.Clock,
) *Server {
	return &Server{
		logger:                  logger,
		workerPool:              workerPool,
		secretManager:           secretManager,
		varSourcePool:           varSourcePool,
		interceptTimeoutFactory: interceptTimeoutFactory,
		interceptUpdateInterval: interceptUpdateInterval,
		containerRepository:     containerRepository,
		destroyer:               destroyer,
		clock:                   clock,
	}
}
