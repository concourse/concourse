package containerserver

import (
	"code.cloudfoundry.org/lager"
	"github.com/concourse/concourse/atc/creds"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/gc"
	"github.com/concourse/concourse/atc/worker"
)

type Server struct {
	logger lager.Logger

	workerPool              worker.Pool
	variablesFactory        creds.VariablesFactory
	interceptTimeoutFactory InterceptTimeoutFactory
	containerRepository     db.ContainerRepository
	destroyer               gc.Destroyer
}

func NewServer(
	logger lager.Logger,
	workerPool worker.Pool,
	variablesFactory creds.VariablesFactory,
	interceptTimeoutFactory InterceptTimeoutFactory,
	containerRepository db.ContainerRepository,
	destroyer gc.Destroyer,
) *Server {
	return &Server{
		logger:                  logger,
		workerPool:              workerPool,
		variablesFactory:        variablesFactory,
		interceptTimeoutFactory: interceptTimeoutFactory,
		containerRepository:     containerRepository,
		destroyer:               destroyer,
	}
}
