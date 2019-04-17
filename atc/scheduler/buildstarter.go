package scheduler

import (
	"code.cloudfoundry.org/lager"
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/engine"
	"github.com/concourse/concourse/atc/scheduler/algorithm"
	"github.com/concourse/concourse/atc/scheduler/maxinflight"
)

//go:generate counterfeiter . BuildStarter

type BuildStarter interface {
	TryStartPendingBuildsForJob(
		logger lager.Logger,
		job db.Job,
		resources db.Resources,
		resourceTypes atc.VersionedResourceTypes,
	) error
}

//go:generate counterfeiter . BuildFactory

type BuildFactory interface {
	Create(atc.JobConfig, atc.ResourceConfigs, atc.VersionedResourceTypes, []db.BuildInput) (atc.Plan, error)
}

func NewBuildStarter(
	pipeline db.Pipeline,
	maxInFlightUpdater maxinflight.Updater,
	factory BuildFactory,
	inputMapper algorithm.InputMapper,
	execEngine engine.Engine,
) BuildStarter {
	return &buildStarter{
		pipeline:           pipeline,
		maxInFlightUpdater: maxInFlightUpdater,
		factory:            factory,
		inputMapper:        inputMapper,
		execEngine:         execEngine,
	}
}

type buildStarter struct {
	pipeline           db.Pipeline
	maxInFlightUpdater maxinflight.Updater
	factory            BuildFactory
	execEngine         engine.Engine
	inputMapper        algorithm.InputMapper
}

func (s *buildStarter) TryStartPendingBuildsForJob(
	logger lager.Logger,
	job db.Job,
	resources db.Resources,
	resourceTypes atc.VersionedResourceTypes,
) error {
	nextPendingBuilds, err := job.GetPendingBuilds()
	if err != nil {
		logger.Error("failed-to-get-all-next-pending-builds", err)
		return err
	}

	for _, nextPendingBuild := range nextPendingBuilds {
		started, err := s.tryStartNextPendingBuild(logger, nextPendingBuild, job, resources, resourceTypes)
		if err != nil {
			return err
		}

		if !started {
			break // stop scheduling next builds after failing to schedule a build
		}
	}

	return nil
}

func (s *buildStarter) tryStartNextPendingBuild(
	logger lager.Logger,
	nextPendingBuild db.Build,
	job db.Job,
	resources db.Resources,
	resourceTypes atc.VersionedResourceTypes,
) (bool, error) {
	logger = logger.Session("try-start-next-pending-build", lager.Data{
		"build-id":   nextPendingBuild.ID(),
		"build-name": nextPendingBuild.Name(),
	})

	reachedMaxInFlight, err := s.maxInFlightUpdater.UpdateMaxInFlightReached(logger, job, nextPendingBuild.ID())
	if err != nil {
		return false, err
	}
	if reachedMaxInFlight {
		return false, nil
	}

	if nextPendingBuild.IsManuallyTriggered() {
		jobBuildInputs := job.Config().Inputs()
		for _, input := range jobBuildInputs {
			resource, found := resources.Lookup(input.Resource)

			if !found {
				logger.Debug("failed-to-find-resource")
				return false, nil
			}

			if resource.CurrentPinnedVersion() != nil {
				continue
			}

			if resource.LastCheckFinished().Before(nextPendingBuild.CreateTime()) {
				return false, nil
			}
		}

		versions, err := s.pipeline.LoadVersionsDB()
		if err != nil {
			logger.Error("failed-to-load-versions-db", err)
			return false, err
		}

		inputMapping, resolved, err := s.inputMapper.MapInputs(versions, job, resources)
		if err != nil {
			return false, err
		}

		err = job.SaveNextInputMapping(inputMapping, resolved)
		if err != nil {
			logger.Error("failed-to-save-next-input-mapping", err)
			return false, err
		}

		dbResourceTypes, err := s.pipeline.ResourceTypes()
		if err != nil {
			return false, err
		}
		resourceTypes = dbResourceTypes.Deserialize()
	}

	buildInputs, found, err := job.GetFullNextBuildInputs()
	if err != nil {
		logger.Error("failed-to-get-next-build-inputs", err)
		return false, err
	}
	if !found {
		return false, nil
	}

	pipelinePaused, err := s.pipeline.CheckPaused()
	if err != nil {
		logger.Error("failed-to-check-if-pipeline-is-paused", err)
		return false, err
	}
	if pipelinePaused {
		return false, nil
	}

	if job.Paused() {
		return false, nil
	}

	updated, err := nextPendingBuild.Schedule(buildInputs)
	if err != nil {
		logger.Error("failed-to-use-inputs", err)
		return false, err
	}

	if !updated {
		logger.Debug("build-already-scheduled")
		return false, nil
	}

	err = nextPendingBuild.AdoptBuildPipes()
	if err != nil {
		logger.Error("failed-to-adopt-build-pipes", err)
		return false, err
	}

	resourceConfigs := atc.ResourceConfigs{}
	for _, v := range resources {
		resourceConfigs = append(resourceConfigs, atc.ResourceConfig{
			Name:   v.Name(),
			Type:   v.Type(),
			Source: v.Source(),
			Tags:   v.Tags(),
		})
	}

	plan, err := s.factory.Create(job.Config(), resourceConfigs, resourceTypes, buildInputs)
	if err != nil {
		// Don't use ErrorBuild because it logs a build event, and this build hasn't started
		err := nextPendingBuild.Finish(db.BuildStatusErrored)
		if err != nil {
			logger.Error("failed-to-mark-build-as-errored", err)
		}
		return false, nil
	}

	createdBuild, err := s.execEngine.CreateBuild(logger, nextPendingBuild, plan)
	if err != nil {
		logger.Error("failed-to-create-build", err)
		return false, nil
	}

	logger.Info("starting")

	go createdBuild.Resume(logger)

	return true, nil
}
