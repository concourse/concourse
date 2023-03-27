package exec

import (
	"context"
	"errors"

	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagerctx"
)

const AbortedLogMessage = "interrupted"
const TimeoutLogMessage = "timeout exceeded"

type LogErrorStep struct {
	Step

	delegateFactory BuildStepDelegateFactory
}

func LogError(step Step, delegateFactory BuildStepDelegateFactory) Step {
	return LogErrorStep{
		Step: step,

		delegateFactory: delegateFactory,
	}
}

func (step LogErrorStep) Run(ctx context.Context, state RunState) (bool, error) {
	logger := lagerctx.FromContext(ctx)

	runOk, runErr := step.Step.Run(ctx, state)
	if runErr == nil {
		return runOk, nil
	}

	var message string
	switch {
	case errors.Is(runErr, context.Canceled):
		message = AbortedLogMessage
	case errors.Is(runErr, context.DeadlineExceeded):
		message = TimeoutLogMessage
	default:
		message = runErr.Error()
	}

	logger.Info("errored", lager.Data{"error": runErr.Error()})

	delegate := step.delegateFactory.BuildStepDelegate(state)
	delegate.Errored(logger, message)

	return runOk, runErr
}
