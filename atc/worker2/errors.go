package worker2

import (
	"errors"
	"fmt"

	"github.com/cppforlife/go-semi-semantic/version"
)

var ErrNoWorkers = errors.New("no workers")

type NoCompatibleWorkersError struct {
	Spec          Spec
	WorkerVersion version.Version
}

func (err NoCompatibleWorkersError) Error() string {
	return fmt.Sprintf("no workers satisfying: %s, version: '%s'", err.Spec.Description(), err.WorkerVersion)
}

type NoWorkerFitContainerPlacementStrategyError struct {
	Strategy string
}

func (err NoWorkerFitContainerPlacementStrategyError) Error() string {
	return fmt.Sprintf("no worker fit container placement strategy: %s", err.Strategy)
}
