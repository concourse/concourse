package exec

import (
	"context"
	"reflect"
	"regexp"

	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagerctx"
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
	re := regexp.MustCompile(`worker .+ disappeared`)
	if re.MatchString(err.Error()) {
		logger.Debug("retry-error",
			lager.Data{"err_type": reflect.TypeOf(err).String(), "err": err})
		return true
	}

	return false
}
