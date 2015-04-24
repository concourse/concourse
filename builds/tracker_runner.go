package builds

import (
	"os"
	"time"

	"github.com/pivotal-golang/clock"
)

//go:generate counterfeiter . BuildTracker

type BuildTracker interface {
	Track()
}

type TrackerRunner struct {
	Tracker  BuildTracker
	Interval time.Duration
	Clock    clock.Clock
}

func (runner TrackerRunner) Run(signals <-chan os.Signal, ready chan<- struct{}) error {
	close(ready)

	runner.Tracker.Track()

	ticker := runner.Clock.NewTicker(runner.Interval)

	for {
		select {
		case <-ticker.C():
			runner.Tracker.Track()
		case <-signals:
			return nil
		}
	}

	panic("unreachable")
}
