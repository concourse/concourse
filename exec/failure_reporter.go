package exec

import "os"

type failureReporter struct {
	Step

	ReportFailure func(error)
}

func (reporter failureReporter) Run(signals <-chan os.Signal, ready chan<- struct{}) error {
	err := reporter.Step.Run(signals, ready)
	if err != nil {
		reporter.ReportFailure(err)
	}

	return err
}
