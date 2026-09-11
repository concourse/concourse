package exec

import (
	"context"

	"code.cloudfoundry.org/lager/v3/lagerctx"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/runtime"
	"github.com/concourse/concourse/atc/worker"
)

// selectStepWorker picks a worker for a step, honoring the build's pinned
// worker (when the job has pin_worker: true).
//
// When no worker is pinned yet, this is the step that picks one (the first
// to call). Subsequent steps are forced to use the same worker.
//
// If two parallel steps both see no pinned worker and both call
// FindOrSelectWorker concurrently, the loser's candidate is released and
// it re-selects on the winner's worker. The check-and-set in
// SetPinnedWorker is atomic, so only one of the parallel steps can win
// the race.
func selectStepWorker(
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
	if !pinWorker {
		return pool.FindOrSelectWorker(
			ctx,
			owner,
			containerSpec,
			spec,
			strategy,
			callback,
		)
	}

	// Fast path: a worker has already been pinned for this build. All
	// subsequent steps in the build, including across substeps, will go
	// through this path.
	if pinned := state.PinnedWorker(); pinned != "" {
		return pool.FindOrSelectWorkerOnPinned(
			ctx,
			owner,
			containerSpec,
			spec,
			pinned,
			strategy,
			callback,
		)
	}

	// Slow path: this is the first step to need a worker in the build.
	// Pick a worker, then try to claim it as the build's pinned worker.
	candidate, err := pool.FindOrSelectWorker(
		ctx,
		owner,
		containerSpec,
		spec,
		strategy,
		callback,
	)
	if err != nil {
		return nil, err
	}
	if candidate == nil {
		return nil, nil
	}
	actualPinned := state.SetPinnedWorker(candidate.Name())
	if actualPinned == candidate.Name() {
		return candidate, nil
	}

	// We lost the race against another parallel step. Release our
	// candidate and re-select on the winning worker so the build ends up
	// on a single worker as promised by pin_worker.
	pool.ReleaseWorker(
		lagerctx.FromContext(ctx),
		containerSpec,
		candidate,
		strategy,
	)
	return pool.FindOrSelectWorkerOnPinned(
		ctx,
		owner,
		containerSpec,
		spec,
		actualPinned,
		strategy,
		callback,
	)
}
