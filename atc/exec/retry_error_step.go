package exec

import (
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagerctx"
	"context"
	"github.com/concourse/concourse/atc/worker/transport"
	"reflect"
)

type Retriable struct{}

func (r Retriable) Error() string {
	return "retriable"
}

type RetryErrorStep struct {
	Step

	delegate LogErrorStepDelegate
}

func RetryError(step Step, delegate LogErrorStepDelegate) Step {
	return RetryErrorStep{
		Step:     LogError(step, delegate),
		delegate: delegate,
	}
}

func (step RetryErrorStep) Run(ctx context.Context, state RunState) error {
	logger := lagerctx.FromContext(ctx)

	runErr := step.Step.Run(ctx, state)
	if runErr != nil && step.toRetry(logger, runErr) {
		logger.Info("retriable", lager.Data{"error": runErr.Error()})
		step.delegate.Errored(logger, "errored, will retry ...")
		runErr = Retriable{}
	}

	return runErr
}

func (step RetryErrorStep) toRetry(logger lager.Logger, err error) bool {
	switch err.(type) {
	case transport.WorkerMissingError, transport.WorkerUnreachableError:
		logger.Debug("retry-error",
			lager.Data{"err_type": reflect.TypeOf(err).String(), "err": err.Error()})
		return true
	// To add: input missing error, because if a previous get/task's worker crashed,
	// later step may hit input missing error, in that, we should rerun the previous
	// get/task. But we should distinct a real input-missing error (bad definition
	// of a pipeline)
	default:
		logger.Debug("non-retry-error",
			lager.Data{"err_type": reflect.TypeOf(err).String(), "err": err.Error()})
		return false
	}
}
