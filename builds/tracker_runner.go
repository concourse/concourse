package builds

import (
	"os"
	"time"

	"code.cloudfoundry.org/clock"
)

//go:generate counterfeiter . BuildTracker

type BuildTracker interface {
	Track()
}

//go:generate counterfeiter . ATCListener

type ATCListener interface {
	Listen(channel string) (chan bool, error)
}

type TrackerRunner struct {
	Tracker   BuildTracker
	ListenBus ATCListener
	Interval  time.Duration
	Clock     clock.Clock
}

func (runner TrackerRunner) Run(signals <-chan os.Signal, ready chan<- struct{}) error {
	ticker := runner.Clock.NewTicker(runner.Interval)
	notify, err := runner.ListenBus.Listen("atc_shutdown")

	if err != nil {
		return err
	}

	close(ready)

	runner.Tracker.Track()

	for {
		select {
		case <-notify:
			runner.Tracker.Track()
		case <-ticker.C():
			runner.Tracker.Track()
		case <-signals:
			return nil
		}
	}

	panic("unreachable")
}
