package lidar

import (
	"code.cloudfoundry.org/lager/lagerctx"
	"context"
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/metric"
	"github.com/concourse/concourse/tracing"
	"math/rand"
	"strconv"
)

func NewScanner(checkFactory db.CheckFactory, planFactory atc.PlanFactory, batchSize int) *scanner {
	return &scanner{
		checkFactory: checkFactory,
		planFactory:  planFactory,
		batchSize:    batchSize,
	}
}

type scanner struct {
	checkFactory db.CheckFactory
	planFactory  atc.PlanFactory
	batchSize    int
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
	cursor := 0
	total := len(resources)
	limit := total
	created := 0
	scanned := 0
	if s.batchSize > 0 && s.batchSize < limit {
		limit = s.batchSize
		cursor = rand.Int() % total
	}

	for created < limit && scanned < total {
		resource := resources[cursor]
		resourceTypes := resourceTypesMap[resource.PipelineID()]
		if s.check(ctx, resource, resourceTypes) {
			created++
		}
		scanned++

		cursor++
		if cursor >= total {
			cursor = 0
		}
	}
}

func (s *scanner) check(ctx context.Context, checkable db.Checkable, resourceTypes db.ResourceTypes) bool {
	logger := lagerctx.FromContext(ctx)

	if checkable.CheckEvery() != nil && checkable.CheckEvery().Never {
		return false
	}

	spanCtx, span := tracing.StartSpan(ctx, "scanner.check", tracing.Attrs{
		"team":                     checkable.TeamName(),
		"pipeline":                 checkable.PipelineName(),
		"resource":                 checkable.Name(),
		"type":                     checkable.Type(),
		"resource_config_scope_id": strconv.Itoa(checkable.ResourceConfigScopeID()),
	})
	defer span.End()

	version := checkable.CurrentPinnedVersion()

	_, created, err := s.checkFactory.TryCreateCheck(lagerctx.NewContext(spanCtx, logger), checkable, resourceTypes, version, false, false, false)
	if err != nil {
		logger.Error("failed-to-create-check", err)
		return false
	}

	if !created {
		logger.Debug("check-already-exists")
	} else {
		metric.Metrics.ChecksEnqueued.Inc()
	}

	return created
}
