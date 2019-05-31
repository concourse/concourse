package radar

import (
	"context"
	"os"
	"sync"
	"time"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/concourse/v5/atc/db"
)

//go:generate counterfeiter . ScanRunnerFactory

type Runner struct {
	logger lager.Logger

	noop bool

	scanRunnerFactory ScanRunnerFactory
	pipeline          db.Pipeline
	syncInterval      time.Duration

	scanning   *sync.Map
	scanningWg *sync.WaitGroup
}

func NewRunner(
	logger lager.Logger,
	noop bool,
	scanRunnerFactory ScanRunnerFactory,
	pipeline db.Pipeline,
	syncInterval time.Duration,
) *Runner {
	return &Runner{
		logger:            logger,
		noop:              noop,
		scanRunnerFactory: scanRunnerFactory,
		pipeline:          pipeline,
		syncInterval:      syncInterval,
		scanning:          &sync.Map{},
		scanningWg:        &sync.WaitGroup{},
	}
}

func (r *Runner) Run(signals <-chan os.Signal, ready chan<- struct{}) error {
	r.logger.Info("start")
	defer r.logger.Info("done")

	close(ready)

	if r.noop {
		<-signals
		return nil
	}

	ticker := time.NewTicker(r.syncInterval)
	scannerContext, cancel := context.WithCancel(context.Background())

	err := r.tick(scannerContext)
	if err != nil {
		cancel()
		return err
	}

	for {
		select {
		case <-ticker.C:
			err := r.tick(scannerContext)
			if err != nil {
				break
			}
		case <-signals:
			ticker.Stop()
			cancel()
			r.scanningWg.Wait()
			return nil
		}
	}
}

func (r *Runner) tick(ctx context.Context) error {
	resourceTypes, err := r.pipeline.ResourceTypes()
	if err != nil {
		r.logger.Error("failed-to-get-resource-types", err)
		return err
	}

	resources, err := r.pipeline.Resources()
	if err != nil {
		r.logger.Error("failed-to-get-resources", err)
		return err
	}

	r.scanResourceTypes(ctx, resourceTypes)
	r.scanResources(ctx, resources)

	return nil
}

func (r *Runner) scanResources(ctx context.Context, resources db.Resources) {
	for _, resource := range resources {
		scopedName := "resource:" + resource.Name()
		if _, found := r.scanning.Load(scopedName); found {
			continue
		}

		logger := r.logger.Session("scan-resource", lager.Data{
			"resource": resource.Name(),
		})

		r.scanningWg.Add(1)
		go func(res db.Resource, scopedName string) {
			defer r.scanningWg.Done()

			r.scanning.Store(scopedName, true)
			runner := r.scanRunnerFactory.ScanResourceRunner(logger, res)
			err := runner.Run(ctx)
			if err != nil {
				r.logger.Error("failed-to-run-scan-resource", err)
			}
			r.scanning.Delete(scopedName)
		}(resource, scopedName)
	}
}

func (r *Runner) scanResourceTypes(ctx context.Context, resourceTypes db.ResourceTypes) {
	for _, resourceType := range resourceTypes {
		scopedName := "resource-type:" + resourceType.Name()
		if _, found := r.scanning.Load(scopedName); found {
			continue
		}

		logger := r.logger.Session("scan-resource-type", lager.Data{
			"resource-type": resourceType.Name(),
		})

		r.scanningWg.Add(1)
		go func(res db.ResourceType, scopedName string) {
			defer r.scanningWg.Done()

			r.scanning.Store(scopedName, true)
			runner := r.scanRunnerFactory.ScanResourceTypeRunner(logger, res)
			err := runner.Run(ctx)
			if err != nil {
				r.logger.Error("failed-to-run-scan-resource-type", err)
			}
			r.scanning.Delete(scopedName)
		}(resourceType, scopedName)
	}
}
