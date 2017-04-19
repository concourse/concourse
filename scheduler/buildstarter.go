package scheduler

import (
	"code.cloudfoundry.org/lager"
	"github.com/concourse/atc"
	"github.com/concourse/atc/config"
	"github.com/concourse/atc/dbng"
	"github.com/concourse/atc/engine"
	"github.com/concourse/atc/scheduler/inputmapper"
	"github.com/concourse/atc/scheduler/maxinflight"
)

//go:generate counterfeiter . BuildStarter

type BuildStarter interface {
	TryStartPendingBuildsForJob(
		logger lager.Logger,
		jobConfig atc.JobConfig,
		resourceConfigs atc.ResourceConfigs,
		resourceTypes atc.VersionedResourceTypes,
		nextPendingBuilds []dbng.Build,
	) error
}

//go:generate counterfeiter . BuildFactory

type BuildFactory interface {
	Create(atc.JobConfig, atc.ResourceConfigs, atc.VersionedResourceTypes, []dbng.BuildInput) (atc.Plan, error)
}

func NewBuildStarter(
	pipeline dbng.Pipeline,
	maxInFlightUpdater maxinflight.Updater,
	factory BuildFactory,
	scanner Scanner,
	inputMapper inputmapper.InputMapper,
	execEngine engine.Engine,
) BuildStarter {
	return &buildStarter{
		pipeline:           pipeline,
		maxInFlightUpdater: maxInFlightUpdater,
		factory:            factory,
		scanner:            scanner,
		inputMapper:        inputMapper,
		execEngine:         execEngine,
	}
}

type buildStarter struct {
	pipeline           dbng.Pipeline
	maxInFlightUpdater maxinflight.Updater
	factory            BuildFactory
	execEngine         engine.Engine
	scanner            Scanner
	inputMapper        inputmapper.InputMapper
}

func (s *buildStarter) TryStartPendingBuildsForJob(
	logger lager.Logger,
	jobConfig atc.JobConfig,
	resourceConfigs atc.ResourceConfigs,
	resourceTypes atc.VersionedResourceTypes,
	nextPendingBuildsForJob []dbng.Build,
) error {
	for _, nextPendingBuild := range nextPendingBuildsForJob {
		started, err := s.tryStartNextPendingBuild(logger, nextPendingBuild, jobConfig, resourceConfigs, resourceTypes)
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
	nextPendingBuild dbng.Build,
	jobConfig atc.JobConfig,
	resourceConfigs atc.ResourceConfigs,
	resourceTypes atc.VersionedResourceTypes,
) (bool, error) {
	logger = logger.Session("try-start-next-pending-build", lager.Data{
		"build-id":   nextPendingBuild.ID(),
		"build-name": nextPendingBuild.Name(),
	})

	reachedMaxInFlight, err := s.maxInFlightUpdater.UpdateMaxInFlightReached(logger, jobConfig, nextPendingBuild.ID())
	if err != nil {
		return false, err
	}
	if reachedMaxInFlight {
		return false, nil
	}

	if nextPendingBuild.IsManuallyTriggered() {
		jobBuildInputs := config.JobInputs(jobConfig)
		for _, input := range jobBuildInputs {
			scanLog := logger.Session("scan", lager.Data{
				"input":    input.Name,
				"resource": input.Resource,
			})

			err := s.scanner.Scan(scanLog, input.Resource)
			if err != nil {
				return false, err
			}
		}

		versions, err := s.pipeline.LoadVersionsDB()
		if err != nil {
			logger.Error("failed-to-load-versions-db", err)
			return false, err
		}

		_, err = s.inputMapper.SaveNextInputMapping(logger, versions, jobConfig)
		if err != nil {
			return false, err
		}
	}

	buildInputs, found, err := s.pipeline.GetNextBuildInputs(nextPendingBuild.JobName())
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

	job, found, err := s.pipeline.Job(nextPendingBuild.JobName())
	if err != nil {
		logger.Error("failed-to-check-if-job-is-paused", err)
		return false, err
	}
	if !found {
		logger.Debug("job-not-found")
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

	plan, err := s.factory.Create(jobConfig, resourceConfigs, resourceTypes, buildInputs)
	if err != nil {
		// Don't use ErrorBuild because it logs a build event, and this build hasn't started
		err := nextPendingBuild.Finish(dbng.BuildStatusErrored)
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
