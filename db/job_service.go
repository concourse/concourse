package db

import (
	"fmt"

	"github.com/concourse/atc"
)

//go:generate counterfeiter . JobServiceDB

type JobServiceDB interface {
	GetJob(job string) (SavedJob, error)
	GetRunningBuildsBySerialGroup(jobName string, serialGroups []string) ([]Build, error)
	GetNextPendingBuildBySerialGroup(jobName string, serialGroups []string) (Build, bool, error)
	UpdateBuildPreparation(prep BuildPreparation) error
	IsPaused() (bool, error)
}

type JobService struct {
	JobConfig atc.JobConfig
	DBJob     SavedJob
	DB        JobServiceDB
}

func NewJobService(config atc.JobConfig, jobServiceDB JobServiceDB) (JobService, error) {
	job, err := jobServiceDB.GetJob(config.Name)
	if err != nil {
		return JobService{}, err
	}

	return JobService{
		JobConfig: config,
		DBJob:     job,
		DB:        jobServiceDB,
	}, nil
}

func (s JobService) updateBuildPrepAndReturn(buildPrep BuildPreparation, scheduled bool, message string) (bool, string, error) {
	err := s.DB.UpdateBuildPreparation(buildPrep)
	if err != nil {
		return false, fmt.Sprintf("update-build-prep-db-failed-when-%s", message), err
	}
	return scheduled, message, nil
}

func (s JobService) CanBuildBeScheduled(build Build, buildPrep BuildPreparation) (bool, string, error) {
	paused, err := s.DB.IsPaused()
	if err != nil {
		return false, "pause-pipeline-db-failed", err
	}

	if paused {
		buildPrep.PausedPipeline = BuildPreparationStatusBlocking
		return s.updateBuildPrepAndReturn(buildPrep, false, "pipeline-paused")
	}

	buildPrep.PausedPipeline = BuildPreparationStatusNotBlocking
	err = s.DB.UpdateBuildPreparation(buildPrep)
	if err != nil {
		return false, "update-build-prep-db-failed-pipeline-not-paused", err
	}

	if build.Scheduled {
		buildPrep.MaxRunningBuilds = BuildPreparationStatusNotBlocking
		return s.updateBuildPrepAndReturn(buildPrep, true, "build-scheduled")
	}

	if s.DBJob.Paused {
		buildPrep.PausedJob = BuildPreparationStatusBlocking
		return s.updateBuildPrepAndReturn(buildPrep, false, "job-paused")
	}

	buildPrep.PausedJob = BuildPreparationStatusNotBlocking
	err = s.DB.UpdateBuildPreparation(buildPrep)
	if err != nil {
		return false, "update-build-prep-db-failed-job-not-paused", err
	}

	if build.Status != StatusPending {
		return false, "build-not-pending", nil
	}

	maxInFlight := s.JobConfig.MaxInFlight()

	if maxInFlight > 0 {
		builds, err := s.DB.GetRunningBuildsBySerialGroup(s.DBJob.Name, s.JobConfig.GetSerialGroups())
		if err != nil {
			return false, "db-failed", err
		}

		if len(builds) >= maxInFlight {
			buildPrep.MaxRunningBuilds = BuildPreparationStatusBlocking
			return s.updateBuildPrepAndReturn(buildPrep, false, "max-in-flight-reached")
		}

		nextMostPendingBuild, found, err := s.DB.GetNextPendingBuildBySerialGroup(s.DBJob.Name, s.JobConfig.GetSerialGroups())
		if err != nil {
			return false, "db-failed", err
		}

		if !found {
			return false, "no-pending-build", nil
		}

		if nextMostPendingBuild.ID != build.ID {
			return false, "not-next-most-pending", nil
		}
	}

	buildPrep.MaxRunningBuilds = BuildPreparationStatusNotBlocking
	err = s.DB.UpdateBuildPreparation(buildPrep)
	if err != nil {
		return false, "update-build-prep-db-failed-not-max-running-builds", err
	}

	return true, "can-be-scheduled", nil
}
