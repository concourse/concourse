package maxinflight

import (
	"code.cloudfoundry.org/lager"
	"github.com/concourse/concourse/atc/db"
	"math"
)

//go:generate counterfeiter . Updater

const (
	defaultMaxContainers       = 250 // The garden default max value
	maxSafeUtilizationFraction = 0.9 // Leave 10% safety buffer
)

type Updater interface {
	UpdateMaxInFlightReached(logger lager.Logger, job db.Job, buildID int) (bool, error)
}

func NewUpdater(pipeline db.Pipeline, workerFactory db.WorkerFactory) Updater {
	return &updater{
		pipeline: pipeline,
		workerFactory: workerFactory,
	}
}

type updater struct {
	pipeline      db.Pipeline
	workerFactory db.WorkerFactory
}

func (u *updater) UpdateMaxInFlightReached(logger lager.Logger, job db.Job, buildID int) (bool, error) {
	logger = logger.Session("is-max-in-flight-reached", lager.Data{"job-name": job.Name()})

	globalReached, err := u.isGlobalMaxInFlightReached(logger)
	if err != nil {
		return false, nil
	}

	if globalReached {
		err = job.SetMaxInFlightReached(globalReached)
		if err != nil {
			logger.Error("failed-to-set-max-in-flight-reached", err)
			return false, err
		}

		return globalReached, nil
	}

	jobReached, err := u.isMaxInFlightReached(logger, job, buildID)
	if err != nil {
		return false, err
	}

	err = job.SetMaxInFlightReached(jobReached)
	if err != nil {
		logger.Error("failed-to-set-max-in-flight-reached", err)
		return false, err
	}

	return jobReached, nil
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

func (u *updater) isGlobalMaxInFlightReached(logger lager.Logger) (bool, error) {
	workers, err := u.workerFactory.Workers()
	if err != nil {
		logger.Error("failed-to-get-workers", err)
		return false, err
	}

	workerCount := len(workers)
	containerMax := defaultMaxContainers * workerCount
	safeContainerMax := int(math.Floor(float64(containerMax) * maxSafeUtilizationFraction))

	containersCurrent := 0
	for _, w := range workers {
		containersCurrent += w.ActiveContainers()
	}

	return containersCurrent >= safeContainerMax, nil
}
