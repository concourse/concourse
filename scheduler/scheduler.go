package scheduler

import (
	"sync"
	"time"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/atc"
	"github.com/concourse/atc/config"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/db/algorithm"
	"github.com/concourse/atc/db/lock"
	"github.com/concourse/atc/scheduler/inputmapper"
)

type Scheduler struct {
	DB           SchedulerDB
	InputMapper  inputmapper.InputMapper
	BuildStarter BuildStarter
	Scanner      Scanner
}

//go:generate counterfeiter . SchedulerDB

type SchedulerDB interface {
	AcquireSchedulingLock(lager.Logger, time.Duration) (lock.Lock, bool, error)
	LoadVersionsDB() (*algorithm.VersionsDB, error)
	GetPipelineName() string
	Reload() (bool, error)
	Config() atc.Config
	CreateJobBuild(job string) (db.Build, error)
	EnsurePendingBuildExists(jobName string) error
	GetAllPendingBuilds() (map[string][]db.Build, error)
	GetPendingBuildsForJob(jobName string) ([]db.Build, error)
}

//go:generate counterfeiter . Scanner

type Scanner interface {
	Scan(lager.Logger, string) error
}

func (s *Scheduler) Schedule(
	logger lager.Logger,
	versions *algorithm.VersionsDB,
	jobConfigs atc.JobConfigs,
	resourceConfigs atc.ResourceConfigs,
	resourceTypes atc.ResourceTypes,
) (map[string]time.Duration, error) {
	jobSchedulingTime := map[string]time.Duration{}

	for _, jobConfig := range jobConfigs {
		jStart := time.Now()
		err := s.ensurePendingBuildExists(logger, versions, jobConfig)
		jobSchedulingTime[jobConfig.Name] = time.Since(jStart)

		if err != nil {
			return jobSchedulingTime, err
		}
	}

	nextPendingBuilds, err := s.DB.GetAllPendingBuilds()
	if err != nil {
		logger.Error("failed-to-get-all-next-pending-builds", err)
		return jobSchedulingTime, err
	}

	for _, jobConfig := range jobConfigs {
		jStart := time.Now()
		nextPendingBuildsForJob, ok := nextPendingBuilds[jobConfig.Name]
		if !ok {
			continue
		}

		err := s.BuildStarter.TryStartPendingBuildsForJob(logger, jobConfig, resourceConfigs, resourceTypes, nextPendingBuildsForJob)
		jobSchedulingTime[jobConfig.Name] = jobSchedulingTime[jobConfig.Name] + time.Since(jStart)

		if err != nil {
			return jobSchedulingTime, err
		}
	}

	return jobSchedulingTime, nil
}

func (s *Scheduler) ensurePendingBuildExists(
	logger lager.Logger,
	versions *algorithm.VersionsDB,
	jobConfig atc.JobConfig,
) error {
	inputMapping, err := s.InputMapper.SaveNextInputMapping(logger, versions, jobConfig)
	if err != nil {
		return err
	}

	for _, inputConfig := range config.JobInputs(jobConfig) {
		inputVersion, ok := inputMapping[inputConfig.Name]

		//trigger: true, and the version has not been used
		if ok && inputVersion.FirstOccurrence && inputConfig.Trigger {
			err := s.DB.EnsurePendingBuildExists(jobConfig.Name)
			if err != nil {
				logger.Error("failed-to-ensure-pending-build-exists", err)
				return err
			}

			break
		}
	}

	return nil
}

type Waiter interface {
	Wait()
}

func (s *Scheduler) TriggerImmediately(
	logger lager.Logger,
	jobConfig atc.JobConfig,
	resourceConfigs atc.ResourceConfigs,
	resourceTypes atc.ResourceTypes,
) (db.Build, Waiter, error) {
	logger = logger.Session("trigger-immediately", lager.Data{"job_name": jobConfig.Name})

	build, err := s.DB.CreateJobBuild(jobConfig.Name)
	if err != nil {
		logger.Error("failed-to-create-job-build", err)
		return nil, nil, err
	}
	wg := new(sync.WaitGroup)
	wg.Add(1)

	go func() {
		defer wg.Done()

		nextPendingBuilds, err := s.DB.GetPendingBuildsForJob(jobConfig.Name)
		if err != nil {
			logger.Error("failed-to-get-next-pending-build-for-job", err)
			return
		}

		err = s.BuildStarter.TryStartPendingBuildsForJob(logger, jobConfig, resourceConfigs, resourceTypes, nextPendingBuilds)
		if err != nil {
			logger.Error("failed-to-start-next-pending-build-for-job", err, lager.Data{"job-name": jobConfig.Name})
			return
		}
	}()

	return build, wg, nil
}

func (s *Scheduler) SaveNextInputMapping(logger lager.Logger, job atc.JobConfig) error {
	versions, err := s.DB.LoadVersionsDB()
	if err != nil {
		logger.Error("failed-to-load-versions-db", err)
		return err
	}

	_, err = s.InputMapper.SaveNextInputMapping(logger, versions, job)
	return err
}
