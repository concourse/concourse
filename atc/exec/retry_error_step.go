package exec

import (
	"context"
	"fmt"
	"github.com/concourse/concourse/atc/worker/transport"
	"reflect"
	"regexp"

	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagerctx"
)

type Retriable struct {
	Cause error
}

func (r Retriable) Error() string {
	return fmt.Sprintf("retriable: %s", r.Cause.Error())
}

type RetryErrorStepDelegate interface {
	Errored(lager.Logger, string)
}

type RetryErrorStep struct {
	Step

	delegate RetryErrorStepDelegate
}

func RetryError(step Step, delegate RetryErrorStepDelegate) Step {
	return RetryErrorStep{
		Step:     step,
		delegate: delegate,
	}
}

func (step RetryErrorStep) Run(ctx context.Context, state RunState) error {
	logger := lagerctx.FromContext(ctx)
	runErr := step.Step.Run(ctx, state)
	if runErr != nil && step.toRetry(logger, runErr) {
		logger.Info("retriable", lager.Data{"error": runErr.Error()})
		step.delegate.Errored(logger, fmt.Sprintf("%s, will retry ...", runErr.Error()))
		runErr = Retriable{runErr}
	}
	return runErr
}

func (step RetryErrorStep) toRetry(logger lager.Logger, err error) bool {
	switch err.(type) {
	case transport.WorkerMissingError, transport.WorkerUnreachableError:
		logger.Debug("retry-error",
			lager.Data{"err_type": reflect.TypeOf(err).String(), "err": err.Error()})
		return true
	default:
		re := regexp.MustCompile(`worker .+ disappeared`)
		if re.MatchString(err.Error()) {
			logger.Debug("retry-error",
				lager.Data{"err_type": reflect.TypeOf(err).String(), "err": err})
			return true
		}
	}
	return false
}
