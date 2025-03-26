package lidar

import (
	"context"
	"strconv"
	"sync"

	"code.cloudfoundry.org/lager/v3"
	"code.cloudfoundry.org/lager/v3/lagerctx"
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/metric"
	"github.com/concourse/concourse/atc/util"
	"github.com/concourse/concourse/tracing"
)

func NewScanner(checkFactory db.CheckFactory, planFactory atc.PlanFactory, maxConcurrency int) *scanner {
	return &scanner{
		checkFactory:   checkFactory,
		planFactory:    planFactory,
		maxConcurrency: maxConcurrency,
	}
}

type scanner struct {
	checkFactory   db.CheckFactory
	planFactory    atc.PlanFactory
	maxConcurrency int
}

func (s *scanner) Run(ctx context.Context) error {
	logger := lagerctx.FromContext(ctx)

	spanCtx, span := tracing.StartSpan(ctx, "scanner.Run", nil)
	defer span.End()

	logger.Info("start")
	defer logger.Info("end")

	resources, err := s.checkFactory.Resources()
	if err != nil {
		logger.Error("failed-to-get-resources", err)
		return err
	}

	resourceTypes, err := s.checkFactory.ResourceTypesByPipeline()
	if err != nil {
		logger.Error("failed-to-get-resource-types", err)
		return err
	}

	s.scanResources(spanCtx, resources, resourceTypes)

	return nil
}

func (s *scanner) scanResources(ctx context.Context, resources []db.Resource, resourceTypesMap map[int]db.ResourceTypes) {
	logger := lagerctx.FromContext(ctx)
	waitGroup := sync.WaitGroup{}

	numberOfResources := len(resources)
	maxConcurrency := min(s.maxConcurrency, numberOfResources)
	resourcesChan := make(chan db.Resource, numberOfResources)

	go func() {
		defer close(resourcesChan)
		for _, rs := range resources {
			select {
			case resourcesChan <- rs:
			case <-ctx.Done():
				logger.Debug("lidar-scanner-cancelled-sending-work", lager.Data{"error": ctx.Err().Error()})
				return
			}
		}
	}()

	for range maxConcurrency {
		waitGroup.Add(1)
		go func() {
			defer waitGroup.Done()
			for {
				select {
				case rs, open := <-resourcesChan:
					if !open {
						// channel closed, no more work to do
						return
					}

					resourceTypes := resourceTypesMap[rs.PipelineID()]

					// Run check inside a func so we don't lose the worker
					// go routine if there's a panic
					func() {
						defer func() {
							err := util.DumpPanic(recover(), "scanning resource %d", rs.ID())
							if err != nil {
								logger.Error("panic-in-scanner-run", err)
							}
						}()
						s.check(ctx, rs, resourceTypes)
					}()

				case <-ctx.Done():
					logger.Debug("lidar-scanner-worker-cancelled", lager.Data{"error": ctx.Err().Error()})
					return
				}
			}
		}()
	}

	done := make(chan struct{})
	go func() {
		waitGroup.Wait()
		close(done)
	}()

	select {
	case <-done:
		return
	case <-ctx.Done():
		logger.Debug("lidar-scanner-cancelled", lager.Data{"error": ctx.Err().Error()})
		return
	}
}

func (s *scanner) check(ctx context.Context, checkable db.Checkable, resourceTypes db.ResourceTypes) {
	logger := lagerctx.FromContext(ctx)

	spanCtx, span := tracing.StartSpan(ctx, "scanner.check", tracing.Attrs{
		"team":                     checkable.TeamName(),
		"pipeline":                 checkable.PipelineName(),
		"resource":                 checkable.Name(),
		"type":                     checkable.Type(),
		"resource_config_scope_id": strconv.Itoa(checkable.ResourceConfigScopeID()),
	})
	defer span.End()

	version := checkable.CurrentPinnedVersion()

	if checkable.CheckEvery() != nil && checkable.CheckEvery().Never {
		return
	}

	_, created, err := s.checkFactory.TryCreateCheck(lagerctx.NewContext(spanCtx, logger), checkable, resourceTypes, version, false, false, false)
	if err != nil {
		logger.Error("failed-to-create-check", err)
		return
	}

	if !created {
		logger.Debug("check-already-exists")
	} else {
		metric.Metrics.ChecksEnqueued.Inc()
	}
}

func (s *scanner) Drain(ctx context.Context) {
	s.checkFactory.Drain()
}
