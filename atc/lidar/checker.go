package lidar

import (
	"context"
	"strconv"
	"sync"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/engine"
	"github.com/concourse/concourse/atc/metric"
	"github.com/concourse/concourse/tracing"
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

	metric.ChecksQueueSize.Set(int64(len(checks)))

	for _, ck := range checks {
		if _, exists := c.running.LoadOrStore(ck.ID(), true); !exists {
			go func(check db.Check) {
				_, span := tracing.StartSpanFollowing(
					check,
					"checker.Run",
					tracing.Attrs{
						"team":                     ck.TeamName(),
						"pipeline":                 ck.PipelineName(),
						"check_id":                 strconv.Itoa(ck.ID()),
						"resource_config_scope_id": strconv.Itoa(ck.ResourceConfigScopeID()),
					},
				)
				defer span.End()
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
