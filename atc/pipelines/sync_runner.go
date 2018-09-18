package pipelines

import (
	"os"
	"time"

	"code.cloudfoundry.org/clock"
)

//go:generate counterfeiter . PipelineSyncer

type PipelineSyncer interface {
	Sync()
}

type SyncRunner struct {
	Syncer   PipelineSyncer
	Interval time.Duration
	Clock    clock.Clock
}

func (runner SyncRunner) Run(signals <-chan os.Signal, ready chan<- struct{}) error {
	ticker := runner.Clock.NewTicker(runner.Interval)

	close(ready)

	runner.Syncer.Sync()

	for {
		select {
		case <-ticker.C():
			runner.Syncer.Sync()
		case <-signals:
			return nil
		}
	}

	panic("unreachable")
}
