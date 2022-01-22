package component

import (
	"context"
	"os"
	"time"

	"code.cloudfoundry.org/clock"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagerctx"
)

var Clock = clock.NewClock()

type NotificationsBus interface {
	Listen(string) (chan bool, error)
	Unlisten(string, chan bool) error
}

// Schedulable represents a workload that is executed normally on a periodic
// schedule, but can also be run immediately.
type Schedulable interface {
	RunPeriodically(context.Context) bool
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

	notifier, err := scheduler.Bus.Listen(scheduler.Component.Name())
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

	interval := scheduler.Interval
	for {
		timer := Clock.NewTimer(interval)

		select {
		case <-notifier:
			timer.Stop()
			runCtx := lagerctx.NewContext(ctx, scheduler.Logger.Session("notify"))
			scheduler.Schedulable.RunImmediately(runCtx)

		case <-timer.C():
			runCtx := lagerctx.NewContext(ctx, scheduler.Logger.Session("tick"))
			hasRun := scheduler.Schedulable.RunPeriodically(runCtx)
			if hasRun {
				interval = scheduler.Interval * 2
			} else {
				if interval > time.Second*2 {
					interval -= time.Second
				}
			}

		case <-ctx.Done():
			timer.Stop()
			return nil
		}
	}
}
