package exec

import (
	"context"
	"fmt"

	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagerctx"
)

// AcrossStep is a step of steps to run in parallel. It behaves the same as InParallelStep
// with the exception that an experimental warning is logged to stderr and that step
// lifecycle build events are emitted (Initializing, Starting, and Finished)
type AcrossStep struct {
	InParallelStep

	delegate BuildStepDelegate
	metadata StepMetadata
}

// Across constructs an AcrossStep.
func Across(
	step InParallelStep,
	delegate BuildStepDelegate,
	metadata StepMetadata,
) AcrossStep {
	return AcrossStep{
		InParallelStep: step,
		delegate:       delegate,
		metadata:       metadata,
	}
}

// Run calls out to InParallelStep.Run after logging a warning to stderr. It also emits
// step lifecycle build events (Initializing, Starting, and Finished).
func (step AcrossStep) Run(ctx context.Context, state RunState) error {
	logger := lagerctx.FromContext(ctx)
	logger = logger.Session("across-step", lager.Data{
		"job-id": step.metadata.JobID,
	})

	step.delegate.Initializing(logger)

	stderr := step.delegate.Stderr()

	fmt.Fprintln(stderr, "\x1b[1;33mWARNING: the across step is experimental and subject to change!\x1b[0m")
	fmt.Fprintln(stderr, "")
	fmt.Fprintln(stderr, "\x1b[33mfollow RFC #29 for updates: https://github.com/concourse/rfcs/pull/29\x1b[0m")
	fmt.Fprintln(stderr, "")

	step.delegate.Starting(logger)

	err := step.InParallelStep.Run(ctx, state)
	if err != nil {
		return err
	}

	step.delegate.Finished(logger, step.Succeeded())

	return nil
}
