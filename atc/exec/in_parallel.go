package exec

import (
	"context"
	"errors"
	"sync/atomic"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/util"
	"github.com/hashicorp/go-multierror"
)

// InParallelStep is a step of steps to run in parallel.
type InParallelStep struct {
	steps       []Step
	maxInFlight atc.MaxInFlightConfig
	failFast    bool
}

// InParallel constructs an InParallelStep.
func InParallel(steps []Step, limit int, failFast bool) InParallelStep {
	maxInFlight := atc.MaxInFlightConfig{Limit: limit}
	if limit < 1 {
		maxInFlight.All = true
	}
	return InParallelStep{
		steps:       steps,
		maxInFlight: maxInFlight,
		failFast:    failFast,
	}
}

// Run executes all steps in order and ensures that the number of running steps
// does not exceed the optional limit to parallelism. By default the limit is equal
// to the number of steps, which means all steps will all be executed in parallel.
//
// Fail fast can be used to abort running steps if any steps exit with an error. When set
// to false, parallel wil wait for all the steps to exit even if a step fails or errors.
//
// Cancelling a parallel step means that any outstanding steps will not be scheduled to run.
// After all steps finish, their errors (if any) will be collected and returned as a
// single error.
func (step InParallelStep) Run(ctx context.Context, state RunState) (bool, error) {
	return parallelExecutor{
		stepName: "in_parallel",

		maxInFlight: &step.maxInFlight,
		failFast:    step.failFast,
		count:       len(step.steps),

		runFunc: func(ctx context.Context, i int) (bool, error) {
			return step.steps[i].Run(ctx, state)
		},
	}.run(ctx)
}

type parallelExecutor struct {
	stepName string

	maxInFlight *atc.MaxInFlightConfig
	failFast    bool
	count       int

	runFunc func(ctx context.Context, i int) (bool, error)
}

func (p parallelExecutor) run(ctx context.Context) (bool, error) {
	var (
		errs          = make(chan error, p.count)
		sem           = make(chan bool, p.maxInFlight.EffectiveLimit(p.count))
		executedSteps int
	)

	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	var numFailures uint32 = 0
	for i := 0; i < p.count; i++ {
		i := i
		sem <- true
		if runCtx.Err() != nil {
			break
		}
		go func() {
			defer func() {
				err := util.DumpPanic(recover(), "%s step", p.stepName)
				if err != nil {
					errs <- err
				}
			}()
			defer func() {
				<-sem
			}()

			succeeded, err := p.runFunc(runCtx, i)
			if !succeeded {
				atomic.AddUint32(&numFailures, 1)
				if p.failFast {
					cancel()
				}
			}
			errs <- err
		}()
		executedSteps++
	}

	var result error
	for i := 0; i < executedSteps; i++ {
		err := <-errs
		if err != nil && !errors.Is(err, context.Canceled) {
			// The Run context being cancelled only means that one or more steps failed, not
			// in_parallel itself. If we return context.Canceled error messages the step will
			// be marked as errored instead of failed, and therefore they should be ignored.
			result = multierror.Append(result, err)
		}
	}

	if ctx.Err() != nil {
		return false, ctx.Err()
	}

	if result != nil {
		return false, result
	}

	allStepsSuccessful := atomic.LoadUint32(&numFailures) == 0
	return allStepsSuccessful, nil
}
