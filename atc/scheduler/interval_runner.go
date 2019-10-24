package scheduler

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
	noop bool,
	runner Runner,
	interval time.Duration,
) ifrit.Runner {
	return &intervalRunner{
		logger:   logger,
		clock:    clock,
		noop:     noop,
		runner:   runner,
		interval: interval,
	}
}

type intervalRunner struct {
	logger   lager.Logger
	runner   Runner
	noop     bool
	clock    clock.Clock
	interval time.Duration
}

func (r *intervalRunner) Run(signals <-chan os.Signal, ready chan<- struct{}) error {
	if r.noop {
		return nil
	}

	close(ready)

	if r.interval == 0 {
		panic("unconfigured scheduler interval")
	}

	r.logger.Info("start", lager.Data{
		"interval": r.interval.String(),
	})

	defer r.logger.Info("done")

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
		case <-signals:
			cancel()
			return nil
		}
	}
}
