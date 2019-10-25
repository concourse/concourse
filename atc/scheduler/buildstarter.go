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

	if nextPendingBuild.IsAborted() {
		return finishCancelledBuild(logger, nextPendingBuild)
	}

	reachedMaxInFlight, err := s.maxInFlightUpdater.UpdateMaxInFlightReached(logger, job, nextPendingBuild.ID())
	if err != nil {
		return false, err
	}
	if reachedMaxInFlight {
		return false, nil
	}

	resourceTypes, prepared, err := s.prepareIfManuallyTriggered(nextPendingBuild, resourceTypes, job, resources, logger)
	if !prepared {
		return false, err
	}

	buildInputs, found, err := getNextBuildInputs(job, logger)
	if !found {
		return false, err
	}

	paused, err := s.isJobOrPipelinePaused(logger, job)
	if paused {
		return false, err
	}

	scheduled, err := schedule(nextPendingBuild, logger)
	if !scheduled {
		return false, err
	}

	err = nextPendingBuild.UseInputs(buildInputs)
	if err != nil {
		return false, err
	}

	return s.start(job, resources, resourceTypes, buildInputs, nextPendingBuild, logger), nil
}

func (s *buildStarter) prepareIfManuallyTriggered(
	nextPendingBuild db.Build,
	resourceTypes atc.VersionedResourceTypes,
	job db.Job, resources db.Resources,
	logger lager.Logger,
) (atc.VersionedResourceTypes, bool, error) {
	prepared := true
	var err error
	if nextPendingBuild.IsManuallyTriggered() {
		resourceTypes, prepared, err = s.prepareManuallyTriggeredBuild(job, resources, logger, nextPendingBuild, resourceTypes)
	}
	return resourceTypes, prepared, err
}

func (s *buildStarter) start(
	job db.Job,
	resources db.Resources,
	resourceTypes atc.VersionedResourceTypes,
	buildInputs []db.BuildInput,
	nextPendingBuild db.Build,
	logger lager.Logger,
) bool {
	plan, err := s.factory.Create(job.Config(), configs(resources), resourceTypes, buildInputs)
	if err != nil {
		// Don't use ErrorBuild because it logs a build event, and this build hasn't started
		if err = nextPendingBuild.Finish(db.BuildStatusErrored); err != nil {
			logger.Error("failed-to-mark-build-as-errored", err)
		}
		return false
	}
	started, err := nextPendingBuild.Start(plan)
	if err != nil {
		logger.Error("failed-to-mark-build-as-started", err)
		return false
	}
	if !started {
		if err = nextPendingBuild.Finish(db.BuildStatusAborted); err != nil {
			logger.Error("failed-to-mark-build-as-finished", err)
		}
		return false
	}
	return true
}

func configs(resources db.Resources) atc.ResourceConfigs {
	resourceConfigs := atc.ResourceConfigs{}
	for _, v := range resources {
		resourceConfigs = append(resourceConfigs, atc.ResourceConfig{
			Name:   v.Name(),
			Type:   v.Type(),
			Source: v.Source(),
			Tags:   v.Tags(),
		})
	}
	return resourceConfigs
}

func schedule(nextPendingBuild db.Build, logger lager.Logger) (bool, error) {
	updated, err := nextPendingBuild.Schedule()
	if err != nil {
		logger.Error("failed-to-update-build-to-scheduled", err)
		return false, err
	}
	if !updated {
		logger.Debug("build-already-scheduled")
		return false, nil
	}
	return true, nil
}

func getNextBuildInputs(job db.Job, logger lager.Logger) ([]db.BuildInput, bool, error) {
	buildInputs, found, err := job.GetNextBuildInputs()
	if err != nil {
		logger.Error("failed-to-get-next-build-inputs", err)
		return nil, false, err
	}
	return buildInputs, found, nil
}

func (s *buildStarter) isJobOrPipelinePaused(logger lager.Logger, job db.Job) (bool, error) {
	pipelinePaused, err := s.pipeline.CheckPaused()
	if err != nil {
		logger.Error("failed-to-check-if-pipeline-is-paused", err)
		return true, err
	}
	if pipelinePaused {
		return true, nil
	}
	if job.Paused() {
		return true, nil
	}
	return false, nil
}

func (s *buildStarter) prepareManuallyTriggeredBuild(job db.Job, resources db.Resources, logger lager.Logger, nextPendingBuild db.Build, resourceTypes atc.VersionedResourceTypes, ) (atc.VersionedResourceTypes, bool, error) {
	if shouldWaitForCheck(job, resources, logger, nextPendingBuild) {
		return nil, false, nil
	}
	err := s.prepareInputMappingForManuallyTriggeredBuild(logger, job, resources)
	if err != nil {
		return nil, false, err
	}
	resourceTypes, err = s.prepareResourceTypesForManuallyTriggeredBuild(err, resourceTypes)
	if err != nil {
		return nil, false, err
	}
	return resourceTypes, true, nil
}

func (s *buildStarter) prepareResourceTypesForManuallyTriggeredBuild(err error, resourceTypes atc.VersionedResourceTypes) (atc.VersionedResourceTypes, error) {
	dbResourceTypes, err := s.pipeline.ResourceTypes()
	if err != nil {
		return nil, err
	}
	return dbResourceTypes.Deserialize(), nil
}

func (s *buildStarter) prepareInputMappingForManuallyTriggeredBuild(logger lager.Logger, job db.Job, resources db.Resources) (error) {
	versions, err := s.pipeline.LoadVersionsDB()
	if err != nil {
		logger.Error("failed-to-load-versions-db", err)
		return err
	}
	_, err = s.inputMapper.SaveNextInputMapping(logger, versions, job, resources)
	if err != nil {
		return err
	}
	return nil
}

func shouldWaitForCheck(job db.Job, resources db.Resources, logger lager.Logger, nextPendingBuild db.Build) (bool) {
	for _, input := range job.Config().Inputs() {
		resource, found := resources.Lookup(input.Resource)

		if !found {
			logger.Debug("failed-to-find-resource")
			return true
		}

		if resource.CurrentPinnedVersion() != nil {
			continue
		}

		if nextPendingBuild.IsNewerThanLastCheckOf(resource) {
			return true
		}
	}
	return false
}

func finishCancelledBuild(logger lager.Logger, nextPendingBuild db.Build) (bool, error) {
	logger.Debug("cancel-aborted-pending-build")
	err := nextPendingBuild.Finish(db.BuildStatusAborted)
	if err != nil {
		return false, err
	}
	return true, nil
}
