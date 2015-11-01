package exec

import "os"

type errorReporter struct {
	Step

	ReportFailure func(error)
}

func (reporter errorReporter) Run(signals <-chan os.Signal, ready chan<- struct{}) error {
	err := reporter.Step.Run(signals, ready)
	if err != nil {
		reporter.ReportFailure(err)
	}

	return err
}
