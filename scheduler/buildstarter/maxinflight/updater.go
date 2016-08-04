package maxinflight

import (
	"code.cloudfoundry.org/lager"
	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
)

//go:generate counterfeiter . Updater

type Updater interface {
	UpdateMaxInFlightReached(logger lager.Logger, jobConfig atc.JobConfig, buildID int) (bool, error)
}

//go:generate counterfeiter . UpdaterDB

type UpdaterDB interface {
	GetRunningBuildsBySerialGroup(jobName string, serialGroups []string) ([]db.Build, error)
	GetNextPendingBuildBySerialGroup(jobName string, serialGroups []string) (db.Build, bool, error)
	SetMaxInFlightReached(jobName string, reached bool) error
}

func NewUpdater(db UpdaterDB) Updater {
	return &updater{db: db}
}

type updater struct {
	db UpdaterDB
}

func (u *updater) UpdateMaxInFlightReached(logger lager.Logger, jobConfig atc.JobConfig, buildID int) (bool, error) {
	logger = logger.Session("is-max-in-flight-reached")
	reached, err := u.isMaxInFlightReached(logger, jobConfig, buildID)
	if err != nil {
		return false, err
	}

	err = u.db.SetMaxInFlightReached(jobConfig.Name, reached)
	if err != nil {
		logger.Error("failed-to-set-max-in-flight-reached", err)
		return false, err
	}

	return reached, nil
}

func (u *updater) isMaxInFlightReached(logger lager.Logger, jobConfig atc.JobConfig, buildID int) (bool, error) {
	maxInFlight := jobConfig.MaxInFlight()

	if maxInFlight == 0 {
		return false, nil
	}

	builds, err := u.db.GetRunningBuildsBySerialGroup(jobConfig.Name, jobConfig.GetSerialGroups())
	if err != nil {
		logger.Error("failed-to-get-running-builds-by-serial-group", err)
		return false, err
	}

	if len(builds) >= maxInFlight {
		return true, nil
	}

	nextMostPendingBuild, found, err := u.db.GetNextPendingBuildBySerialGroup(jobConfig.Name, jobConfig.GetSerialGroups())
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
