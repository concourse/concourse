package scheduler

import (
	"fmt"

	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
)

//go:generate counterfeiter . JobServiceDB

type JobServiceDB interface {
	GetJob(job string) (db.SavedJob, error)
	GetRunningBuildsBySerialGroup(jobName string, serialGroups []string) ([]db.Build, error)
	GetNextPendingBuildBySerialGroup(jobName string, serialGroups []string) (db.Build, bool, error)
	UpdateBuildPreparation(prep db.BuildPreparation) error
	IsPaused() (bool, error)
}

//go:generate counterfeiter . JobService

type JobService interface {
	CanBuildBeScheduled(build db.Build, buildPrep *db.BuildPreparation) (bool, string, error)
}

type jobService struct {
	JobConfig atc.JobConfig
	DBJob     db.SavedJob
	DB        JobServiceDB
}

func NewJobService(config atc.JobConfig, jobServiceDB JobServiceDB) (JobService, error) {
	job, err := jobServiceDB.GetJob(config.Name)
	if err != nil {
		return jobService{}, err
	}

	return jobService{
		JobConfig: config,
		DBJob:     job,
		DB:        jobServiceDB,
	}, nil
}

func (s jobService) updateBuildPrepAndReturn(buildPrep db.BuildPreparation, scheduled bool, message string) (bool, string, error) {
	err := s.DB.UpdateBuildPreparation(buildPrep)
	if err != nil {
		return false, fmt.Sprintf("update-build-prep-db-failed-when-%s", message), err
	}
	return scheduled, message, nil
}

func (s jobService) CanBuildBeScheduled(build db.Build, buildPrep *db.BuildPreparation) (bool, string, error) {
	paused, err := s.DB.IsPaused()
	if err != nil {
		return false, "pause-pipeline-db-failed", err
	}

	if paused {
		buildPrep.PausedPipeline = db.BuildPreparationStatusBlocking
		return s.updateBuildPrepAndReturn(*buildPrep, false, "pipeline-paused")
	}

	buildPrep.PausedPipeline = db.BuildPreparationStatusNotBlocking
	err = s.DB.UpdateBuildPreparation(*buildPrep)
	if err != nil {
		return false, "update-build-prep-db-failed-pipeline-not-paused", err
	}

	if build.Scheduled {
		buildPrep.MaxRunningBuilds = db.BuildPreparationStatusNotBlocking
		return s.updateBuildPrepAndReturn(*buildPrep, true, "build-scheduled")
	}

	if s.DBJob.Paused {
		buildPrep.PausedJob = db.BuildPreparationStatusBlocking
		return s.updateBuildPrepAndReturn(*buildPrep, false, "job-paused")
	}

	buildPrep.PausedJob = db.BuildPreparationStatusNotBlocking
	err = s.DB.UpdateBuildPreparation(*buildPrep)
	if err != nil {
		return false, "update-build-prep-db-failed-job-not-paused", err
	}

	if build.Status != db.StatusPending {
		return false, "build-not-pending", nil
	}

	maxInFlight := s.JobConfig.MaxInFlight()

	if maxInFlight > 0 {
		builds, err := s.DB.GetRunningBuildsBySerialGroup(s.DBJob.Name, s.JobConfig.GetSerialGroups())
		if err != nil {
			return false, "db-failed", err
		}

		if len(builds) >= maxInFlight {
			buildPrep.MaxRunningBuilds = db.BuildPreparationStatusBlocking
			return s.updateBuildPrepAndReturn(*buildPrep, false, "max-in-flight-reached")
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

	buildPrep.MaxRunningBuilds = db.BuildPreparationStatusNotBlocking
	err = s.DB.UpdateBuildPreparation(*buildPrep)
	if err != nil {
		return false, "update-build-prep-db-failed-not-max-running-builds", err
	}

	return true, "can-be-scheduled", nil
}
