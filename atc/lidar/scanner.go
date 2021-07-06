package lidar

import (
	"code.cloudfoundry.org/lager/lagerctx"
	"context"
	"encoding/json"
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/component"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/metric"
	"github.com/concourse/concourse/atc/util"
	"github.com/concourse/concourse/tracing"
	"strconv"
	"sync"
)

func NewScanner(checkFactory db.CheckFactory, planFactory atc.PlanFactory, chunks int) *scanner {
	return &scanner{
		checkFactory: checkFactory,
		planFactory:  planFactory,
		chunks:       chunks,
	}
}

type scannerRunResult struct {
	LastScannedResourceId int `json:"last_scanned_resource_id"`
}

func parseLastScanResult(result string) scannerRunResult {
	sr := scannerRunResult{
		LastScannedResourceId: -1,
	}
	if result == "" {
		return sr
	}

	// Ignore json decode error. If the field is never set or crashes, it will
	// be auto fixed in next run.
	json.Unmarshal([]byte(result), &sr)
	return sr
}

func (r scannerRunResult) String() string {
	b, err := json.Marshal(r)
	if err != nil {
		return ""
	}
	return string(b)
}

type scanner struct {
	checkFactory db.CheckFactory
	planFactory  atc.PlanFactory
	chunks       int
}

// Run processes 1/chunks of resources and resource types in each period, which
// helps distribute Lidar checks to different ATCs. The last scanned resource id
// and resource type id are stored in the component last run result.
func (s *scanner) Run(ctx context.Context, lastRunResult string) (component.RunResult, error) {
	logger := lagerctx.FromContext(ctx)

	spanCtx, span := tracing.StartSpan(ctx, "scanner.Run", nil)
	defer span.End()

	logger.Info("start")
	defer logger.Info("end")

	resources, err := s.checkFactory.Resources()
	if err != nil {
		logger.Error("failed-to-get-resources", err)
		return nil, err
	}

	resourceTypes, err := s.checkFactory.ResourceTypesByPipeline()
	if err != nil {
		logger.Error("failed-to-get-resource-types", err)
		return nil, err
	}

	scanResult := parseLastScanResult(lastRunResult)
	scanResult.LastScannedResourceId = s.scanResources(spanCtx, resources, resourceTypes, scanResult.LastScannedResourceId)

	return scanResult, nil
}

func (s *scanner) scanResources(ctx context.Context, resources []db.Resource, resourceTypesMap map[int]db.ResourceTypes, lastScannedId int) int {
	if len(resources) == 0 {
		return -1
	}

	logger := lagerctx.FromContext(ctx)
	waitGroup := new(sync.WaitGroup)

	totalSize := len(resources)
	batchSize := 1 + totalSize/s.chunks
	if batchSize > totalSize {
		batchSize = totalSize
	}

	index := 0
	for index < totalSize && lastScannedId > resources[index].ID() {
		index++
	}

	for i := 0; i < batchSize; i++ {
		if index >= totalSize {
			index = 0
		}
		resource := resources[index]
		lastScannedId = resource.ID()
		index++

		waitGroup.Add(1)

		resourceTypes := resourceTypesMap[resource.PipelineID()]
		go func(r db.Resource, rts db.ResourceTypes) {
			defer func() {
				err := util.DumpPanic(recover(), "scanning resource %d", r.ID())
				if err != nil {
					logger.Error("panic-in-scanner-run", err)
				}
			}()
			defer waitGroup.Done()

			s.check(ctx, r, rts)
		}(resource, resourceTypes)
	}
	waitGroup.Wait()

	return lastScannedId
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
