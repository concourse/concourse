package lidar

import (
	"context"
	"os"
	"time"

	"code.cloudfoundry.org/clock"
	"code.cloudfoundry.org/lager"
	"github.com/concourse/concourse/atc/db"
	"github.com/tedsuo/ifrit"
)

//go:generate counterfeiter . Runner

type Runner interface {
	Run(context.Context) error
}

//go:generate counterfeiter . Notifications

type Notifications interface {
	Listen(string) (chan bool, error)
	Unlisten(string, chan bool) error
}

func NewIntervalRunner(
	logger lager.Logger,
	clock clock.Clock,
	runner Runner,
	interval time.Duration,
	notifications Notifications,
	channel string,
	componentFactory db.ComponentFactory,
) ifrit.Runner {
	return &intervalRunner{
		logger:           logger,
		clock:            clock,
		runner:           runner,
		interval:         interval,
		notifications:    notifications,
		channel:          channel,
		componentFactory: componentFactory,
	}
}

type intervalRunner struct {
	logger           lager.Logger
	clock            clock.Clock
	runner           Runner
	interval         time.Duration
	notifications    Notifications
	channel          string
	componentFactory db.ComponentFactory
}

func (r *intervalRunner) Run(signals <-chan os.Signal, ready chan<- struct{}) error {
	r.logger.Info("start")
	defer r.logger.Info("done")

	close(ready)

	notifier, err := r.notifications.Listen(r.channel)
	if err != nil {
		return err
	}

	defer r.notifications.Unlisten(r.channel, notifier)

	ticker := r.clock.NewTicker(r.interval)
	ctx, cancel := context.WithCancel(context.Background())

	if err := r.runner.Run(ctx); err != nil {
		r.logger.Error("failed-to-run", err)
	}

	for {
		select {
		case <-ticker.C():
			component, _, err := r.componentFactory.Find(r.channel)
			if err != nil {
				r.logger.Error("failed-to-find-component", err)
				break
			}

			if component.Paused() {
				r.logger.Debug("component-is-paused", lager.Data{"name": r.channel})
				break
			}

			if !component.IntervalElapsed() {
				r.logger.Debug("component-interval-not-reached", lager.Data{"name": r.channel, "last-ran": component.LastRan()})
				break
			}

			if err := r.runner.Run(ctx); err != nil {
				r.logger.Error("failed-to-run", err)
			}

			if err = component.UpdateLastRan(); err != nil {
				r.logger.Error("failed-to-update-last-ran", err)
			}
		case <-notifier:
			if err := r.runner.Run(ctx); err != nil {
				r.logger.Error("failed-to-run", err)
			}
		case <-signals:
			cancel()
			return nil
		}
	}
}
