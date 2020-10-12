package exec

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/url"
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

type RetryErrorStep struct {
	Step

	delegateFactory BuildStepDelegateFactory
}

func RetryError(step Step, delegateFactory BuildStepDelegateFactory) Step {
	return RetryErrorStep{
		Step:            step,
		delegateFactory: delegateFactory,
	}
}

func (step RetryErrorStep) Run(ctx context.Context, state RunState) (bool, error) {
	logger := lagerctx.FromContext(ctx)
	runOk, runErr := step.Step.Run(ctx, state)
	if runErr != nil && step.toRetry(logger, runErr) {
		logger.Info("retriable", lager.Data{"error": runErr.Error()})
		delegate := step.delegateFactory.BuildStepDelegate(state)
		delegate.Errored(logger, fmt.Sprintf("%s, will retry ...", runErr.Error()))
		runErr = Retriable{runErr}
	}
	return runOk, runErr
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
