package buildstarter

import (
	"code.cloudfoundry.org/lager"
	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/engine"
	"github.com/concourse/atc/scheduler/buildstarter/maxinflight"
)

//go:generate counterfeiter . BuildStarter

type BuildStarter interface {
	TryStartAllPendingBuilds(
		logger lager.Logger,
		jobConfig atc.JobConfig,
		resourceConfigs atc.ResourceConfigs,
		resourceTypes atc.ResourceTypes,
	) error
}

//go:generate counterfeiter . BuildStarterDB

type BuildStarterDB interface {
	GetNextPendingBuild(job string) (db.Build, bool, error)
	GetNextBuildInputs(jobName string) ([]db.BuildInput, bool, error)
	IsPaused() (bool, error)
	GetJob(job string) (db.SavedJob, error)
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
}

func (s *buildStarter) TryStartAllPendingBuilds(
	logger lager.Logger,
	jobConfig atc.JobConfig,
	resourceConfigs atc.ResourceConfigs,
	resourceTypes atc.ResourceTypes,
) error {
	started := true
	for started {
		var err error
		started, err = s.tryStartNextPendingBuild(logger, jobConfig, resourceConfigs, resourceTypes)
		if err != nil {
			return err
		}
	}

	return nil
}

func (s *buildStarter) tryStartNextPendingBuild(
	logger lager.Logger,
	jobConfig atc.JobConfig,
	resourceConfigs atc.ResourceConfigs,
	resourceTypes atc.ResourceTypes,
) (bool, error) {
	logger = logger.Session("try-start-next-pending-build")

	nextPendingBuild, found, err := s.db.GetNextPendingBuild(jobConfig.Name)
	if err != nil {
		logger.Error("failed-to-get-next-pending-build", err)
		return false, err
	}
	if !found {
		return false, nil
	}

	logger = logger.WithData(lager.Data{
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

	buildInputs, found, err := s.db.GetNextBuildInputs(jobConfig.Name)
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

	job, err := s.db.GetJob(jobConfig.Name)
	if err != nil {
		logger.Error("failed-to-check-if-job-is-paused", err)
		return false, err
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
