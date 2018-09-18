package maxinflight

import (
	"code.cloudfoundry.org/lager"
	"github.com/concourse/atc/db"
)

//go:generate counterfeiter . Updater

type Updater interface {
	UpdateMaxInFlightReached(logger lager.Logger, job db.Job, buildID int) (bool, error)
}

func NewUpdater(pipeline db.Pipeline) Updater {
	return &updater{pipeline: pipeline}
}

type updater struct {
	pipeline db.Pipeline
}

func (u *updater) UpdateMaxInFlightReached(logger lager.Logger, job db.Job, buildID int) (bool, error) {
	logger = logger.Session("is-max-in-flight-reached", lager.Data{"job-name": job.Name()})

	reached, err := u.isMaxInFlightReached(logger, job, buildID)
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

func (u *updater) isMaxInFlightReached(logger lager.Logger, job db.Job, buildID int) (bool, error) {
	maxInFlight := job.Config().MaxInFlight()

	if maxInFlight == 0 {
		return false, nil
	}

	builds, err := job.GetRunningBuildsBySerialGroup(job.Config().GetSerialGroups())
	if err != nil {
		logger.Error("failed-to-get-running-builds-by-serial-group", err)
		return false, err
	}

	if len(builds) >= maxInFlight {
		return true, nil
	}

	nextMostPendingBuild, found, err := job.GetNextPendingBuildBySerialGroup(job.Config().GetSerialGroups())
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
