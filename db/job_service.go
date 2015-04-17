package db

import "github.com/concourse/atc"

//go:generate counterfeiter . JobServiceDB

type JobServiceDB interface {
	GetBuild(buildID int) (Build, error)
	GetJob(job string) (Job, error)
	GetRunningBuildsByJob(job string) ([]Build, error)
	GetNextPendingBuild(job string) (Build, []BuildInput, error)
}

type JobService struct {
	JobConfig atc.JobConfig
	DBJob     Job
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

func (s JobService) CanBuildBeScheduled(build Build) (bool, string, error) {
	if s.DBJob.Paused {
		return false, "job-paused", nil
	}

	if build.Status != StatusPending {
		return false, "build-not-pending", nil
	}

	if s.JobConfig.IsSerial() {
		builds, err := s.DB.GetRunningBuildsByJob(s.DBJob.Name)
		if err != nil {
			return false, "db-failed", err
		}

		if len(builds) > 0 {
			return false, "other-builds-running", nil
		}

		nextMostPendingBuild, _, err := s.DB.GetNextPendingBuild(s.DBJob.Name)
		if err != nil {
			return false, "db-failed", err
		}

		if nextMostPendingBuild.ID != build.ID {
			return false, "not-next-most-pending", nil
		}
	}

	return true, "can-be-scheduled", nil
}
