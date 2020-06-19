package component

import (
	"context"
	"os"
	"time"

	"code.cloudfoundry.org/clock"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagerctx"
	"github.com/concourse/concourse/atc/db"
)

var Clock = clock.NewClock()

type NotificationsBus interface {
	Listen(string, db.NotificationQueueMode) (chan db.Notification, error)
	Unlisten(string, chan db.Notification) error
}

// Schedulable represents a workload that is executed normally on a periodic
// schedule, but can also be run immediately.
type Schedulable interface {
	RunPeriodically(context.Context)
	RunImmediately(context.Context)
}

// Runner runs a workload periodically, or immediately upon receiving a
// notification.
type Runner struct {
	Logger lager.Logger

	Interval  time.Duration
	Component Component
	Bus       NotificationsBus

	Schedulable Schedulable
}

func (scheduler *Runner) Run(signals <-chan os.Signal, ready chan<- struct{}) error {
	scheduler.Logger.Debug("start")
	defer scheduler.Logger.Debug("done")

	notifier, err := scheduler.Bus.Listen(scheduler.Component.Name(), db.DontQueueNotifications)
	if err != nil {
		return err
	}

	defer scheduler.Bus.Unlisten(scheduler.Component.Name(), notifier)

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		<-signals
		cancel()
	}()

	close(ready)

	for {
		timer := Clock.NewTimer(scheduler.Interval)

		select {
		case <-notifier:
			timer.Stop()
			runCtx := lagerctx.NewContext(ctx, scheduler.Logger.Session("notify"))
			scheduler.Schedulable.RunImmediately(runCtx)

		case <-timer.C():
			runCtx := lagerctx.NewContext(ctx, scheduler.Logger.Session("tick"))
			scheduler.Schedulable.RunPeriodically(runCtx)

		case <-ctx.Done():
			timer.Stop()
			return nil
		}
	}
}
