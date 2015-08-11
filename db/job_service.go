package db

import "github.com/concourse/atc"

//go:generate counterfeiter . JobServiceDB

type JobServiceDB interface {
	GetJob(job string) (SavedJob, error)
	GetRunningBuildsBySerialGroup(jobName string, serialGroups []string) ([]Build, error)
	GetNextPendingBuildBySerialGroup(jobName string, serialGroups []string) (Build, error)
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

func (s JobService) CanBuildBeScheduled(build Build) (bool, string, error) {
	if s.DBJob.Paused {
		return false, "job-paused", nil
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
			return false, "max-in-flight-reached", nil
		}

		nextMostPendingBuild, err := s.DB.GetNextPendingBuildBySerialGroup(s.DBJob.Name, s.JobConfig.GetSerialGroups())
		if err != nil {
			return false, "db-failed", err
		}

		if nextMostPendingBuild.ID != build.ID {
			return false, "not-next-most-pending", nil
		}
	}

	return true, "can-be-scheduled", nil
}
