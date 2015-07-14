package exec

import (
	"fmt"
	"os"
	"strings"

	"github.com/hashicorp/go-multierror"
)

type hookedCompose struct {
	step    StepFactory
	failure StepFactory
	success StepFactory
	ensure  StepFactory
	next    StepFactory

	prev Step
	repo *SourceRepository

	firstStep   Step
	failureStep Step
	successStep Step
	ensureStep  Step
	nextStep    Step
}

func HookedCompose(
	step StepFactory,
	next StepFactory,
	failure StepFactory,
	success StepFactory,
	ensure StepFactory,
) StepFactory {
	return hookedCompose{
		step:    step,
		next:    next,
		failure: failure,
		success: success,
		ensure:  ensure,
	}
}

func (hc hookedCompose) Using(prev Step, repo *SourceRepository) Step {
	hc.repo = repo
	hc.prev = prev
	return &hc
}

func (hc *hookedCompose) Run(signals <-chan os.Signal, ready chan<- struct{}) error {
	hc.firstStep = hc.step.Using(hc.prev, hc.repo)

	firstStepError := hc.firstStep.Run(signals, ready)

	var succeeded Success

	if !hc.firstStep.Result(&succeeded) {
		succeeded = false
	}

	var errors error
	var hooks []Step

	if firstStepError == nil {
		if bool(succeeded) {
			hc.successStep = hc.success.Using(hc.firstStep, hc.repo)
			hooks = append(hooks, hc.successStep)
		} else {
			hc.failureStep = hc.failure.Using(hc.firstStep, hc.repo)
			hooks = append(hooks, hc.failureStep)
		}
	} else {
		errors = multierror.Append(errors, firstStepError)
	}

	hc.ensureStep = hc.ensure.Using(hc.firstStep, hc.repo)
	hooks = append(hooks, hc.ensureStep)

	var allHooksSuccessful Success
	allHooksSuccessful = true

	for _, hook := range hooks {
		hookError := hook.Run(signals, make(chan struct{}))
		if hookError != nil {
			errors = multierror.Append(errors, hookError)
		}

		var hookSuccessful Success

		if !hook.Result(&hookSuccessful) {
			allHooksSuccessful = false
		}

		if !bool(hookSuccessful) {
			allHooksSuccessful = false
		}
	}

	if errors != nil {
		return errors
	}

	if bool(succeeded) && bool(allHooksSuccessful) {
		hc.nextStep = hc.next.Using(hc.firstStep, hc.repo)
		return hc.nextStep.Run(signals, make(chan struct{}))
	} else {
		noop := &NoopStep{}
		return noop.Run(signals, make(chan struct{}))
	}

}

func (hc *hookedCompose) Release() error {
	errorMessages := []string{}
	if hc.firstStep != nil {
		if err := hc.firstStep.Release(); err != nil {
			errorMessages = append(errorMessages, "first step: "+err.Error())
		}
	}

	if hc.ensureStep != nil {
		if err := hc.ensureStep.Release(); err != nil {
			errorMessages = append(errorMessages, "ensure step: "+err.Error())
		}
	}

	if hc.failureStep != nil {
		if err := hc.failureStep.Release(); err != nil {
			errorMessages = append(errorMessages, "failure step: "+err.Error())
		}
	}

	if hc.successStep != nil {
		if err := hc.successStep.Release(); err != nil {
			errorMessages = append(errorMessages, "success step: "+err.Error())
		}
	}

	if hc.nextStep != nil {
		if err := hc.nextStep.Release(); err != nil {
			errorMessages = append(errorMessages, "next step: "+err.Error())
		}
	}

	if len(errorMessages) > 0 {
		return fmt.Errorf("sources failed to release:\n%s", strings.Join(errorMessages, "\n"))
	}

	return nil
}

func (hc *hookedCompose) Result(x interface{}) bool {
	switch v := x.(type) {
	case *Success:
		if hc.nextStep == nil {
			*v = false
			return true
		}

		if !hc.nextStep.Result(v) {
			*v = false
		}

		return true
	}

	return false
}
