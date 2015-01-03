package exec

import (
	"os"
	"sync"
)

type SuccessReporter interface {
	Subject(Step) Step

	Successful() bool
}

func NewSuccessReporter() SuccessReporter {
	return &successReporter{
		successful: true,
	}
}

type successReporter struct {
	successful  bool
	successfulL sync.Mutex
}

func (reporter *successReporter) Subject(step Step) Step {
	return successReporterStep{
		reporter: reporter,
		step:     step,
	}
}

func (reporter *successReporter) Successful() bool {
	reporter.successfulL.Lock()
	defer reporter.successfulL.Unlock()

	return reporter.successful
}

func (reporter *successReporter) fail() {
	reporter.successfulL.Lock()
	defer reporter.successfulL.Unlock()

	reporter.successful = false
}

type successReporterStep struct {
	reporter *successReporter

	step Step

	ArtifactSource
}

func (step successReporterStep) Using(source ArtifactSource) ArtifactSource {
	step.ArtifactSource = step.step.Using(source)
	return &step
}

func (step successReporterStep) Run(signals <-chan os.Signal, ready chan<- struct{}) error {
	err := step.ArtifactSource.Run(signals, ready)
	if err != nil {
		step.reporter.fail()
		return err
	}

	if indicator, ok := step.ArtifactSource.(SuccessIndicator); ok {
		if !indicator.Successful() {
			step.reporter.fail()
		}
	}

	return nil
}
