package exec

import (
	"context"

	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagerctx"
)

const AbortedLogMessage = "interrupted"
const TimeoutLogMessage = "timeout exceeded"

type LogErrorStep struct {
	Step

	delegate BuildStepDelegate
}

func LogError(step Step, delegate BuildStepDelegate) Step {
	return LogErrorStep{
		Step: step,

		delegate: delegate,
	}
}

func (step LogErrorStep) Run(ctx context.Context, state RunState) error {
	logger := lagerctx.FromContext(ctx)

	runErr := step.Step.Run(ctx, state)

	var message string
	switch runErr {
	case nil:
		return nil
	case context.Canceled:
		message = AbortedLogMessage
	case context.DeadlineExceeded:
		message = TimeoutLogMessage
	default:
		message = runErr.Error()
	}

	logger.Info("errored", lager.Data{"error": runErr.Error()})

	step.delegate.Errored(logger, message)

	return runErr
}
