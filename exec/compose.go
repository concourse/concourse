package exec

import (
	"fmt"
	"os"
	"strings"

	"github.com/tedsuo/ifrit"
)

func Compose(a StepFactory, b StepFactory) StepFactory {
	return composed{a: a, b: b}
}

type composed struct {
	a StepFactory
	b StepFactory

	prev Step
	repo *SourceRepository

	firstStep  Step
	secondStep Step
}

func (step composed) Using(prev Step, repo *SourceRepository) Step {
	step.prev = prev
	step.repo = repo
	return &step
}

func (step *composed) Run(signals <-chan os.Signal, ready chan<- struct{}) error {
	step.firstStep = step.a.Using(step.prev, step.repo)

	firstProcess := ifrit.Background(step.firstStep)

	var signalled bool
	var waitErr error

dance:
	for {
		select {
		case waitErr = <-firstProcess.Wait():
			break dance

		case sig := <-signals:
			firstProcess.Signal(sig)
			signalled = true
		}
	}

	if signalled || waitErr != nil {
		return waitErr
	}

	step.secondStep = step.b.Using(step.firstStep, step.repo)

	return step.secondStep.Run(signals, ready)
}

func (step *composed) Release() error {
	errorMessages := []string{}

	if step.firstStep != nil {
		if err := step.firstStep.Release(); err != nil {
			errorMessages = append(errorMessages, "first step: "+err.Error())
		}
	}

	if step.secondStep != nil {
		if err := step.secondStep.Release(); err != nil {
			errorMessages = append(errorMessages, "second step: "+err.Error())
		}
	}

	if len(errorMessages) > 0 {
		return fmt.Errorf("sources failed to release:\n%s", strings.Join(errorMessages, "\n"))
	}

	return nil
}

func (step *composed) Result(x interface{}) bool {
	return step.secondStep.Result(x)
}
