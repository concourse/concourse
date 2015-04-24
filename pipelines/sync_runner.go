package pipelines

import (
	"os"
	"time"

	"github.com/pivotal-golang/clock"
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
	close(ready)

	runner.Syncer.Sync()

	ticker := runner.Clock.NewTicker(runner.Interval)

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
