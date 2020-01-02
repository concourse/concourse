package lidar

import (
	"context"
	"sync"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/engine"
	"github.com/concourse/concourse/atc/metric"
)

func NewChecker(
	logger lager.Logger,
	checkFactory db.CheckFactory,
	engine engine.Engine,
) *checker {
	return &checker{
		logger:       logger,
		checkFactory: checkFactory,
		engine:       engine,
		running:      &sync.Map{},
	}
}

type checker struct {
	logger lager.Logger

	checkFactory db.CheckFactory
	engine       engine.Engine

	running *sync.Map
}

func (c *checker) Run(ctx context.Context) error {
	c.logger.Info("start")
	defer c.logger.Info("end")

	checks, err := c.checkFactory.StartedChecks()
	if err != nil {
		c.logger.Error("failed-to-fetch-resource-checks", err)
		return err
	}

	metric.CheckQueueSize{
		Checks: len(checks),
	}.Emit(c.logger)

	for _, ck := range checks {
		if _, exists := c.running.LoadOrStore(ck.ID(), true); !exists {
			go func(check db.Check) {
				defer c.running.Delete(check.ID())

				engineCheck := c.engine.NewCheck(check)
				engineCheck.Run(c.logger.WithData(lager.Data{
					"check": check.ID(),
				}))
			}(ck)
		}
	}

	return nil
}
