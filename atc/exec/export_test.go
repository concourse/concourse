package exec

import (
	"context"

	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/runtime"
	"github.com/concourse/concourse/atc/worker"
)

// SelectStepWorkerForTest exposes the unexported selectStepWorker for
// tests in the exec_test package.
func SelectStepWorkerForTest(
	ctx context.Context,
	pool Pool,
	strategy worker.PlacementStrategy,
	pinWorker bool,
	state RunState,
	callback worker.PoolCallback,
	owner db.ContainerOwner,
	containerSpec runtime.ContainerSpec,
	spec worker.Spec,
) (runtime.Worker, error) {
	return selectStepWorker(
		ctx,
		pool,
		strategy,
		pinWorker,
		state,
		callback,
		owner,
		containerSpec,
		spec,
	)
}
