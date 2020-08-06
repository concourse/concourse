package exec

import (
	"context"
	"errors"
	"fmt"
	"io"
	"reflect"
	"regexp"

	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagerctx"
	"github.com/concourse/concourse/atc/worker/transport"
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
		logger.Info("retryable", lager.Data{"error": runErr.Error()})
		step.delegate.Errored(logger, fmt.Sprintf("%s, will retry ...", runErr.Error()))
		runErr = Retriable{runErr}
	}
	return runErr
}

func (step RetryErrorStep) toRetry(logger lager.Logger, err error) bool {
	var transportErr *transport.WorkerMissingError
	var unreachable *transport.WorkerUnreachableError
	re := regexp.MustCompile(`worker .+ disappeared`)
	if ok := errors.As(err, transportErr) || errors.As(err, unreachable); ok {
		logger.Debug("retry-error",
			lager.Data{"err_type": reflect.TypeOf(err).String(), "err": err.Error()})
		return true
	} else if re.MatchString(err.Error()) {
		logger.Debug("retry-error",
			lager.Data{"err_type": reflect.TypeOf(err).String(), "err": err})
		return true
	} else if errors.Is(err, io.EOF) {
		logger.Debug("retry-error",
			lager.Data{"err_type": reflect.TypeOf(err).String(), "err": err.Error()})
		return true

	}
	return false
}
