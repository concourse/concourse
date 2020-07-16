package lidar

import (
	"context"
	"fmt"
	"os"
	"runtime/debug"
	"strconv"
	"sync"

	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagerctx"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/engine"
	"github.com/concourse/concourse/atc/metric"
	"github.com/concourse/concourse/tracing"
)

//go:generate counterfeiter . RateCalculator

type RateCalculator interface {
	RateLimiter() (Limiter, error)
}

func NewChecker(
	logger lager.Logger,
	checkFactory db.CheckFactory,
	engine engine.Engine,
	checkRateCalculator RateCalculator,
) *checker {
	return &checker{
		logger:              logger,
		checkFactory:        checkFactory,
		engine:              engine,
		running:             &sync.Map{},
		checkRateCalculator: checkRateCalculator,
	}
}

type checker struct {
	logger lager.Logger

	checkFactory        db.CheckFactory
	engine              engine.Engine
	checkRateCalculator RateCalculator

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

	if len(checks) == 0 {
		return nil
	}

	limiter, err := c.checkRateCalculator.RateLimiter()
	if err != nil {
		return err
	}

	for _, ck := range checks {
		if _, exists := c.running.LoadOrStore(ck.ID(), true); !exists {
			if !ck.ManuallyTriggered() {
				err := limiter.Wait(ctx)
				if err != nil {
					c.logger.Error("failed-to-wait-for-limiter", err)
					continue
				}
			}

			go func(check db.Check) {
				loggerData := lager.Data{
					"check_id": strconv.Itoa(check.ID()),
				}
				defer func() {
					if r := recover(); r != nil {
						err = fmt.Errorf("panic in checker check run %s: %v", loggerData, r)

						fmt.Fprintf(os.Stderr, "%s\n %s\n", err.Error(), string(debug.Stack()))
						c.logger.Error("panic-in-checker-check-run", err)

						check.FinishWithError(err)
					}
				}()

				spanCtx, span := tracing.StartSpanFollowing(
					ctx,
					check,
					"checker.Run",
					tracing.Attrs{
						"team":                     check.TeamName(),
						"pipeline":                 check.PipelineName(),
						"check_id":                 strconv.Itoa(check.ID()),
						"resource_config_scope_id": strconv.Itoa(check.ResourceConfigScopeID()),
					},
				)
				defer span.End()
				defer c.running.Delete(check.ID())

				c.engine.NewCheck(check).Run(
					lagerctx.NewContext(
						spanCtx,
						c.logger.WithData(loggerData),
					),
				)
			}(ck)
		}
	}

	return nil
}
