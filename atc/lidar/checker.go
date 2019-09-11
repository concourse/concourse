package lidar

import (
	"context"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/engine"
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
	}
}

type checker struct {
	logger lager.Logger

	checkFactory db.CheckFactory
	engine       engine.Engine
}

func (c *checker) Run(ctx context.Context) error {
	cLog := c.logger.Session("check")

	cLog.Debug("start")
	defer cLog.Debug("done")

	checks, err := c.checkFactory.StartedChecks()
	if err != nil {
		c.logger.Error("failed-to-fetch-resource-checks", err)
		return err
	}

	for _, check := range checks {
		btLog := cLog.WithData(lager.Data{
			"check": check.ID(),
		})

		engineCheck := c.engine.NewCheck(check)
		go engineCheck.Run(btLog)
	}

	return nil
}
