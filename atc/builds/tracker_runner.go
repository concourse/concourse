package builds

import (
	"os"
	"time"

	"code.cloudfoundry.org/clock"
	"code.cloudfoundry.org/lager"
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db"
)

//go:generate counterfeiter . BuildTracker

type BuildTracker interface {
	Track()
	Release()
}

//go:generate counterfeiter . Notifications

type Notifications interface {
	Listen(channel string) (chan bool, error)
	Unlisten(channel string, notifier chan bool) error
	Notify(channel string) error
}

type TrackerRunner struct {
	Tracker          BuildTracker
	Notifications    Notifications
	Interval         time.Duration
	Clock            clock.Clock
	Logger           lager.Logger
	ComponentFactory db.ComponentFactory
}

func (runner TrackerRunner) Run(signals <-chan os.Signal, ready chan<- struct{}) error {
	close(ready)

	shutdownNotifier, err := runner.Notifications.Listen("atc_shutdown")
	if err != nil {
		return err
	}

	defer runner.Notifications.Unlisten("atc_shutdown", shutdownNotifier)

	buildNotifier, err := runner.Notifications.Listen("build_started")
	if err != nil {
		return err
	}

	defer runner.Notifications.Unlisten("build_started", buildNotifier)

	ticker := runner.Clock.NewTicker(runner.Interval)

	for {
		select {
		case <-ticker.C():
			component, _, err := runner.ComponentFactory.Find(atc.ComponentBuildTracker)
			if err != nil {
				runner.Logger.Error("failed-to-find-component", err)
				break
			}

			if component.Paused() {
				runner.Logger.Debug("component-is-paused", lager.Data{"name": atc.ComponentBuildTracker})
				break
			}

			if !component.IntervalElapsed() {
				runner.Logger.Debug("component-interval-not-reached", lager.Data{"name": atc.ComponentBuildTracker, "last-ran": component.LastRan()})
				break
			}

			runner.Tracker.Track()

			if err = component.UpdateLastRan(); err != nil {
				runner.Logger.Error("failed-to-update-last-ran", err)
			}

		case <-shutdownNotifier:
			runner.Logger.Info("received-atc-shutdown-message")
			component, _, err := runner.ComponentFactory.Find(atc.ComponentBuildTracker)
			if err != nil {
				runner.Logger.Error("failed-to-find-component", err)
				break
			}

			if component.Paused() {
				runner.Logger.Debug("component-is-paused", lager.Data{"name": atc.ComponentBuildTracker})
				break
			}

			runner.Tracker.Track()

			if err = component.UpdateLastRan(); err != nil {
				runner.Logger.Error("failed-to-update-last-ran", err)
			}

		case <-buildNotifier:
			runner.Logger.Info("received-build-started-message")
			component, _, err := runner.ComponentFactory.Find(atc.ComponentBuildTracker)
			if err != nil {
				runner.Logger.Error("failed-to-find-component", err)
				break
			}

			if component.Paused() {
				runner.Logger.Debug("component-is-paused", lager.Data{"name": atc.ComponentBuildTracker})
				break
			}

			runner.Tracker.Track()

			if err = component.UpdateLastRan(); err != nil {
				runner.Logger.Error("failed-to-update-last-ran", err)
			}

		case <-signals:
			runner.Logger.Info("releasing-tracker")
			runner.Tracker.Release()
			runner.Logger.Info("released-tracker")
			runner.Logger.Info("sending-atc-shutdown-message")
			return runner.Notifications.Notify("atc_shutdown")
		}
	}
}
