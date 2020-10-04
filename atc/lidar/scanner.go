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
	"github.com/concourse/concourse/atc/metric"
	"github.com/concourse/concourse/tracing"
	"github.com/pkg/errors"
)

func NewScanner(checkFactory db.CheckFactory) *scanner {
	return &scanner{
		checkFactory: checkFactory,
	}
}

type scanner struct {
	checkFactory db.CheckFactory
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

	resourceTypes, err := s.checkFactory.ResourceTypes()
	if err != nil {
		logger.Error("failed-to-get-resource-types", err)
		return err
	}

	waitGroup := new(sync.WaitGroup)
	resourceTypesChecked := &sync.Map{}

	for _, resource := range resources {
		waitGroup.Add(1)

		go func(resource db.Resource, resourceTypes db.ResourceTypes) {
			loggerData := lager.Data{
				"resource_id":   strconv.Itoa(resource.ID()),
				"resource_name": resource.Name(),
				"pipeline_name": resource.PipelineName(),
				"team_name":     resource.TeamName(),
			}
			defer func() {
				if r := recover(); r != nil {
					err = fmt.Errorf("panic in scanner run %s: %v", loggerData, r)

					fmt.Fprintf(os.Stderr, "%s\n %s\n", err.Error(), string(debug.Stack()))
					logger.Error("panic-in-scanner-run", err)

					s.setCheckError(logger, resource, err)
				}
			}()
			defer waitGroup.Done()

			err := s.check(spanCtx, resource, resourceTypes, resourceTypesChecked)
			s.setCheckError(logger, resource, err)

		}(resource, resourceTypes)
	}

	waitGroup.Wait()

	return s.checkFactory.NotifyChecker()
}

func (s *scanner) check(ctx context.Context, checkable db.Checkable, resourceTypes db.ResourceTypes, resourceTypesChecked *sync.Map) error {
	logger := lagerctx.FromContext(ctx)

	spanCtx, span := tracing.StartSpan(ctx, "scanner.check", tracing.Attrs{
		"team":                     checkable.TeamName(),
		"pipeline":                 checkable.PipelineName(),
		"resource":                 checkable.Name(),
		"type":                     checkable.Type(),
		"resource_config_scope_id": strconv.Itoa(checkable.ResourceConfigScopeID()),
	})
	defer span.End()

	parentType, found := resourceTypes.Parent(checkable)
	if found {
		if _, exists := resourceTypesChecked.LoadOrStore(parentType.ID(), true); !exists {
			// only create a check for resource type if it has not been checked yet
			err := s.check(spanCtx, parentType, resourceTypes, resourceTypesChecked)
			s.setCheckError(logger, parentType, err)

			if err != nil {
				logger.Error("failed-to-create-type-check", err)
				return errors.Wrapf(err, "parent type '%v' error", parentType.Name())
			}
		}
	}

	version := checkable.CurrentPinnedVersion()

	_, created, err := s.checkFactory.TryCreateCheck(lagerctx.NewContext(spanCtx, logger), checkable, resourceTypes, version, false)
	if err != nil {
		logger.Error("failed-to-create-check", err)
		return err
	}

	if !created {
		logger.Debug("check-already-exists")
	}

	metric.Metrics.ChecksEnqueued.Inc()

	return nil
}

func (s *scanner) setCheckError(logger lager.Logger, checkable db.Checkable, err error) {
	setErr := checkable.SetCheckSetupError(err)
	if setErr != nil {
		logger.Error("failed-to-set-check-error", setErr)
	}
}
