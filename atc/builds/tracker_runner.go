package builds

import (
	"os"
	"time"

	"code.cloudfoundry.org/clock"
	"code.cloudfoundry.org/lager"
)

//go:generate counterfeiter . BuildTracker

type BuildTracker interface {
	Track()
	Release()
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
	DrainCh   <-chan struct{}
	Logger    lager.Logger
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
		case <-runner.DrainCh:
			return nil
		case <-notify:
			runner.Logger.Info("received-atc-shutdown-message")
			runner.Tracker.Track()
		case <-ticker.C():
			runner.Tracker.Track()
		case <-signals:
			return nil
		}
	}

	panic("unreachable")
}
