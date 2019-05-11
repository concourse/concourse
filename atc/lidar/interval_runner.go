package lidar

import (
	"context"
	"os"
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
	}
}

type intervalRunner struct {
	logger   lager.Logger
	clock    clock.Clock
	runner   Runner
	interval time.Duration
	notifier chan bool
}

func (r *intervalRunner) Run(signals <-chan os.Signal, ready chan<- struct{}) error {
	r.logger.Info("start")
	defer r.logger.Info("done")

	close(ready)

	ticker := r.clock.NewTicker(r.interval)
	ctx, cancel := context.WithCancel(context.Background())

	if err := r.runner.Run(ctx); err != nil {
		r.logger.Error("failed-to-run", err)
	}

	for {
		select {
		case <-ticker.C():
			if err := r.runner.Run(ctx); err != nil {
				r.logger.Error("failed-to-run", err)
			}
		case <-r.notifier:
			if err := r.runner.Run(ctx); err != nil {
				r.logger.Error("failed-to-run", err)
			}
		case <-signals:
			cancel()
			return nil
		}
	}
}
