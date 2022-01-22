package component

import (
	"context"
	"math/rand"
	"os"
	"time"

	"code.cloudfoundry.org/clock"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagerctx"
	"github.com/concourse/concourse/atc/db"
)

var Clock = clock.NewClock()

type NotificationsBus interface {
	Listen(string, int) (chan db.Notification, error)
	Unlisten(string, chan db.Notification) error
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

	notifier, err := scheduler.Bus.Listen(scheduler.Component.Name(), 1)
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
		drift := time.Duration(250 - rand.Int() % 500) * time.Millisecond
		timer := Clock.NewTimer(interval + drift)

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
				if interval > scheduler.Interval {
					interval -= 2 * time.Second // FIXME: should be interval/count_of_atc
				}
			}

		case <-ctx.Done():
			timer.Stop()
			return nil
		}
	}
}
