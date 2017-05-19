package maxinflight

import (
	"code.cloudfoundry.org/lager"
	"github.com/concourse/atc"
	"github.com/concourse/atc/dbng"
)

//go:generate counterfeiter . Updater

type Updater interface {
	UpdateMaxInFlightReached(logger lager.Logger, jobConfig atc.JobConfig, buildID int) (bool, error)
}

func NewUpdater(pipeline dbng.Pipeline) Updater {
	return &updater{pipeline: pipeline}
}

type updater struct {
	pipeline dbng.Pipeline
}

func (u *updater) UpdateMaxInFlightReached(logger lager.Logger, jobConfig atc.JobConfig, buildID int) (bool, error) {
	logger = logger.Session("is-max-in-flight-reached", lager.Data{"job-name": jobConfig.Name})

	job, found, err := u.pipeline.Job(jobConfig.Name)
	if err != nil {
		logger.Error("failed-to-get-job", err)
		return false, err
	}

	if !found {
		logger.Info("job-not-found")
		return true, nil
	}

	reached, err := u.isMaxInFlightReached(logger, jobConfig, buildID, job)
	if err != nil {
		return false, err
	}

	err = job.SetMaxInFlightReached(reached)
	if err != nil {
		logger.Error("failed-to-set-max-in-flight-reached", err)
		return false, err
	}

	return reached, nil
}

func (u *updater) isMaxInFlightReached(logger lager.Logger, jobConfig atc.JobConfig, buildID int, job dbng.Job) (bool, error) {
	maxInFlight := jobConfig.MaxInFlight()

	if maxInFlight == 0 {
		return false, nil
	}

	builds, err := job.GetRunningBuildsBySerialGroup(jobConfig.GetSerialGroups())
	if err != nil {
		logger.Error("failed-to-get-running-builds-by-serial-group", err)
		return false, err
	}

	if len(builds) >= maxInFlight {
		return true, nil
	}

	nextMostPendingBuild, found, err := job.GetNextPendingBuildBySerialGroup(jobConfig.GetSerialGroups())
	if err != nil {
		logger.Error("failed-to-get-next-pending-build-by-serial-group", err)
		return false, err
	}

	if !found {
		logger.Info("pending-build-disappeared-from-serial-group")
		return true, nil
	}

	return nextMostPendingBuild.ID() != buildID, nil
}
