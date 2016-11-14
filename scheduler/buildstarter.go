package scheduler

import (
	"code.cloudfoundry.org/lager"
	"github.com/concourse/atc"
	"github.com/concourse/atc/config"
	"github.com/concourse/atc/db"
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
		resourceTypes atc.ResourceTypes,
		nextPendingBuilds []db.Build,
		scanner Scanner,
		sdb SchedulerDB,
		inputMapper inputmapper.InputMapper,

	) error
}

//go:generate counterfeiter . BuildStarterDB

type BuildStarterDB interface {
	GetNextBuildInputs(jobName string) ([]db.BuildInput, bool, error)
	IsPaused() (bool, error)
	GetJob(job string) (db.SavedJob, bool, error)
	UpdateBuildToScheduled(int) (bool, error)
	UseInputsForBuild(buildID int, inputs []db.BuildInput) error
}

//go:generate counterfeiter . BuildStarterBuildsDB

type BuildStarterBuildsDB interface {
	FinishBuild(buildID int, pipelineID int, status db.Status) error
}

//go:generate counterfeiter . BuildFactory

type BuildFactory interface {
	Create(atc.JobConfig, atc.ResourceConfigs, atc.ResourceTypes, []db.BuildInput) (atc.Plan, error)
}

func NewBuildStarter(
	db BuildStarterDB,
	maxInFlightUpdater maxinflight.Updater,
	factory BuildFactory,
	execEngine engine.Engine,
) BuildStarter {
	return &buildStarter{
		db:                 db,
		maxInFlightUpdater: maxInFlightUpdater,
		factory:            factory,
		execEngine:         execEngine,
	}
}

type buildStarter struct {
	db                 BuildStarterDB
	maxInFlightUpdater maxinflight.Updater
	factory            BuildFactory
	execEngine         engine.Engine
	schedulerDB        SchedulerDB
	inputMapper        inputmapper.InputMapper
}

func (s *buildStarter) TryStartPendingBuildsForJob(
	logger lager.Logger,
	jobConfig atc.JobConfig,
	resourceConfigs atc.ResourceConfigs,
	resourceTypes atc.ResourceTypes,
	nextPendingBuildsForJob []db.Build,
	scanner Scanner,
	sdb SchedulerDB,
	inputMapper inputmapper.InputMapper,
) error {
	for _, nextPendingBuild := range nextPendingBuildsForJob {
		started, err := s.tryStartNextPendingBuild(logger, nextPendingBuild, jobConfig, resourceConfigs, resourceTypes, scanner, sdb, inputMapper)
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
	jobConfig atc.JobConfig,
	resourceConfigs atc.ResourceConfigs,
	resourceTypes atc.ResourceTypes,
	scanner Scanner,
	sdb SchedulerDB,
	inputMapper inputmapper.InputMapper,
) (bool, error) {
	logger = logger.Session("try-start-next-pending-build", lager.Data{
		"build-id":   nextPendingBuild.ID(),
		"build-name": nextPendingBuild.Name(),
	})

	if nextPendingBuild.IsManuallyTriggered() {
		jobBuildInputs := config.JobInputs(jobConfig)
		for _, input := range jobBuildInputs {
			scanLog := logger.Session("scan", lager.Data{
				"input":    input.Name,
				"resource": input.Resource,
			})

			err := scanner.Scan(scanLog, input.Resource)
			if err != nil {
				return false, err
			}
		}

		versions, err := sdb.LoadVersionsDB()
		if err != nil {
			logger.Error("failed-to-load-versions-db", err)
			return false, err
		}

		_, err = inputMapper.SaveNextInputMapping(logger, versions, jobConfig)
		if err != nil {
			return false, err
		}
	}

	reachedMaxInFlight, err := s.maxInFlightUpdater.UpdateMaxInFlightReached(logger, jobConfig, nextPendingBuild.ID())
	if err != nil {
		return false, err
	}
	if reachedMaxInFlight {
		return false, nil
	}

	buildInputs, found, err := s.db.GetNextBuildInputs(nextPendingBuild.JobName())
	if err != nil {
		logger.Error("failed-to-get-next-build-inputs", err)
		return false, err
	}
	if !found {
		return false, nil
	}

	pipelinePaused, err := s.db.IsPaused()
	if err != nil {
		logger.Error("failed-to-check-if-pipeline-is-paused", err)
		return false, err
	}
	if pipelinePaused {
		return false, nil
	}

	job, found, err := s.db.GetJob(nextPendingBuild.JobName())
	if err != nil {
		logger.Error("failed-to-check-if-job-is-paused", err)
		return false, err
	}
	if !found {
		logger.Debug("job-not-found")
		return false, nil
	}
	if job.Paused {
		return false, nil
	}

	updated, err := s.db.UpdateBuildToScheduled(nextPendingBuild.ID())
	if err != nil {
		logger.Error("failed-to-update-build-to-scheduled", err)
		return false, err
	}

	if !updated {
		logger.Debug("build-already-scheduled")
		return false, nil
	}

	err = s.db.UseInputsForBuild(nextPendingBuild.ID(), buildInputs)
	if err != nil {
		return false, err
	}

	plan, err := s.factory.Create(jobConfig, resourceConfigs, resourceTypes, buildInputs)
	if err != nil {
		// Don't use ErrorBuild because it logs a build event, and this build hasn't started
		err := nextPendingBuild.Finish(db.StatusErrored)
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
