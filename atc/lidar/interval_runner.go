package lidar

import (
	"context"
	"os"
	"sync"
	"time"

	"code.cloudfoundry.org/clock"
	"code.cloudfoundry.org/lager"
	"github.com/tedsuo/ifrit"
)

//go:generate counterfeiter . Runner

type Runner interface {
	Run(context.Context) error
}

func NewIntervalRunner(
	logger lager.Logger,
	clock clock.Clock,
	runner Runner,
	interval time.Duration,
	notifier chan bool,
) ifrit.Runner {
	return &intervalRunner{
		logger,
		clock,
		runner,
		interval,
		notifier,
		&sync.WaitGroup{},
	}
}

type intervalRunner struct {
	logger    lager.Logger
	clock     clock.Clock
	runner    Runner
	interval  time.Duration
	notifier  chan bool
	waitGroup *sync.WaitGroup
}

func (r *intervalRunner) Run(signals <-chan os.Signal, ready chan<- struct{}) error {
	r.logger.Info("start")
	defer r.logger.Info("done")

	close(ready)

	ticker := r.clock.NewTicker(r.interval)
	ctx, cancel := context.WithCancel(context.Background())

	if err := r.run(ctx); err != nil {
		r.logger.Error("failed-to-run", err)
	}

	for {
		select {
		case <-ticker.C():
			if err := r.run(ctx); err != nil {
				r.logger.Error("failed-to-run", err)
			}
		case <-r.notifier:
			if err := r.run(ctx); err != nil {
				r.logger.Error("failed-to-run", err)
			}
		case <-signals:
			cancel()
			r.waitGroup.Wait()
			return nil
		}
	}
}

func (r *intervalRunner) run(ctx context.Context) error {
	r.waitGroup.Add(1)
	defer r.waitGroup.Done()

	return r.runner.Run(ctx)
}
