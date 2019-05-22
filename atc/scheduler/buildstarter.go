package scheduler

import (
	"code.cloudfoundry.org/lager"
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/scheduler/inputmapper"
	"github.com/concourse/concourse/atc/scheduler/maxinflight"
)

//go:generate counterfeiter . BuildStarter

type BuildStarter interface {
	TryStartPendingBuildsForJob(
		logger lager.Logger,
		job db.Job,
		resources db.Resources,
		resourceTypes atc.VersionedResourceTypes,
		nextPendingBuilds []db.Build,
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
	inputMapper inputmapper.InputMapper,
) BuildStarter {
	return &buildStarter{
		pipeline:           pipeline,
		maxInFlightUpdater: maxInFlightUpdater,
		factory:            factory,
		inputMapper:        inputMapper,
	}
}

type buildStarter struct {
	pipeline           db.Pipeline
	maxInFlightUpdater maxinflight.Updater
	factory            BuildFactory
	inputMapper        inputmapper.InputMapper
}

func (s *buildStarter) TryStartPendingBuildsForJob(
	logger lager.Logger,
	job db.Job,
	resources db.Resources,
	resourceTypes atc.VersionedResourceTypes,
	nextPendingBuildsForJob []db.Build,
) error {
	for _, nextPendingBuild := range nextPendingBuildsForJob {
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
		for _, input := range job.Config().Inputs() {
			resource, found := resources.Lookup(input.Resource)

			if !found {
				logger.Debug("failed-to-find-resource")
				return false, nil
			}

			if resource.CurrentPinnedVersion() != nil {
				continue
			}

			if resource.LastCheckEndTime().Before(nextPendingBuild.CreateTime()) {
				return false, nil
			}
		}

		versions, err := s.pipeline.LoadVersionsDB()
		if err != nil {
			logger.Error("failed-to-load-versions-db", err)
			return false, err
		}

		_, err = s.inputMapper.SaveNextInputMapping(logger, versions, job, resources)
		if err != nil {
			return false, err
		}

		dbResourceTypes, err := s.pipeline.ResourceTypes()
		if err != nil {
			return false, err
		}
		resourceTypes = dbResourceTypes.Deserialize()
	}

	buildInputs, found, err := job.GetNextBuildInputs()
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

	updated, err := nextPendingBuild.Schedule()
	if err != nil {
		logger.Error("failed-to-update-build-to-scheduled", err)
		return false, err
	}

	if !updated {
		logger.Debug("build-already-scheduled")
		return false, nil
	}

	err = nextPendingBuild.UseInputs(buildInputs)
	if err != nil {
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
		if err = nextPendingBuild.Finish(db.BuildStatusErrored); err != nil {
			logger.Error("failed-to-mark-build-as-errored", err)
		}
		return false, nil
	}

	started, err := nextPendingBuild.Start(plan)
	if err != nil {
		logger.Error("failed-to-mark-build-as-started", err)
		return false, nil
	}

	if !started {
		if err = nextPendingBuild.Finish(db.BuildStatusAborted); err != nil {
			logger.Error("failed-to-mark-build-as-finished", err)
		}
		return false, nil
	}

	return true, nil
}
