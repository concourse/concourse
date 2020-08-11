package exec

import (
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagerctx"
	"context"
	"errors"
	"fmt"
	"github.com/concourse/concourse/atc/worker/transport"
	"net"
	"net/url"
	"reflect"
	"regexp"
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
	var urlError *url.Error
	var netError net.Error
	if errors.As(err, &transport.WorkerMissingError{}) || errors.As(err, &transport.WorkerUnreachableError{}) || errors.As(err, &urlError) {
		logger.Debug("retry-error",
			lager.Data{"err_type": reflect.TypeOf(err).String(), "err": err.Error()})
		return true
	} else if errors.As(err, &netError) || regexp.MustCompile(`worker .+ disappeared`).MatchString(err.Error()) {
		logger.Debug("retry-error",
			lager.Data{"err_type": reflect.TypeOf(err).String(), "err": err})
		return true
	}
	return false
}
