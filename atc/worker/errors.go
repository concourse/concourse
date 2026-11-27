package worker

import (
	"errors"
	"fmt"
	"time"

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

type WaitingForWorkerTimeoutError struct {
	Spec    Spec
	Timeout time.Duration
}

func (err WaitingForWorkerTimeoutError) Error() string {
	return fmt.Sprintf("timed out after %s waiting for a worker satisfying: %s", err.Timeout, err.Spec.Description())
}

type NoWorkerFitContainerPlacementStrategyError struct {
	Strategy string
}

func (err NoWorkerFitContainerPlacementStrategyError) Error() string {
	return fmt.Sprintf("no worker fit container placement strategy: %s", err.Strategy)
}

type StreamingResourceCacheNotFoundError struct {
	Handle          string
	ResourceCacheID int
}

func (e StreamingResourceCacheNotFoundError) Error() string {
	return fmt.Sprintf("resource cache not found (id %d, volume handle %s)", e.ResourceCacheID, e.Handle)
}
