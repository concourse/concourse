package exec

import (
	"context"
	"errors"
	"fmt"
	"os"
	"runtime/debug"
	"strings"
)

// InParallelStep is a step of steps to run in parallel.
type InParallelStep struct {
	steps    []Step
	limit    int
	failFast bool
}

// InParallel constructs an InParallelStep.
func InParallel(steps []Step, limit int, failFast bool) InParallelStep {
	if limit < 1 {
		limit = len(steps)
	}
	return InParallelStep{
		steps:    steps,
		limit:    limit,
		failFast: failFast,
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
func (step InParallelStep) Run(ctx context.Context, state RunState) error {
	var (
		errs          = make(chan error, len(step.steps))
		sem           = make(chan bool, step.limit)
		executedSteps int
	)

	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	for _, s := range step.steps {
		s := s
		sem <- true

		if runCtx.Err() != nil {
			break
		}

		go func() {
			defer func() {
				if r := recover(); r != nil {
					err := fmt.Errorf("panic in parallel step: %v", r)

					fmt.Fprintf(os.Stderr, "%s\n %s\n", err.Error(), string(debug.Stack()))
					errs <- err
				}
			}()
			defer func() {
				<-sem
			}()

			errs <- s.Run(runCtx, state)
			if !s.Succeeded() && step.failFast {
				cancel()
			}
		}()
		executedSteps++
	}

	var errorMessages []string
	for i := 0; i < executedSteps; i++ {
		err := <-errs
		if err != nil && !errors.Is(err, context.Canceled) {
			// The Run context being cancelled only means that one or more steps failed, not
			// in_parallel itself. If we return context.Canceled error messages the step will
			// be marked as errored instead of failed, and therefore they should be ignored.
			errorMessages = append(errorMessages, err.Error())
		}
	}

	if ctx.Err() != nil {
		return ctx.Err()
	}

	if len(errorMessages) > 0 {
		return fmt.Errorf("one or more parallel steps errored:\n%s", strings.Join(errorMessages, "\n"))
	}

	return nil
}

// Succeeded is true if all of the steps' Succeeded is true
func (step InParallelStep) Succeeded() bool {
	succeeded := true

	for _, step := range step.steps {
		if !step.Succeeded() {
			succeeded = false
		}
	}

	return succeeded
}
