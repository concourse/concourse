package scheduler

import (
	"sync"
	"time"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/atc"
	"github.com/concourse/atc/config"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/db/algorithm"
	"github.com/concourse/atc/scheduler/buildstarter"
	"github.com/concourse/atc/scheduler/inputmapper"
)

type Scheduler struct {
	DB           SchedulerDB
	InputMapper  inputmapper.InputMapper
	BuildStarter buildstarter.BuildStarter
	Scanner      Scanner
}

//go:generate counterfeiter . SchedulerDB

type SchedulerDB interface {
	AcquireSchedulingLock(lager.Logger, time.Duration) (db.Lock, bool, error)
	LoadVersionsDB() (*algorithm.VersionsDB, error)
	GetPipelineName() string
	Reload() (bool, error)
	Config() atc.Config
	CreateJobBuild(job string) (db.Build, error)
	EnsurePendingBuildExists(jobName string) error
	AcquireResourceCheckingForJobLock(logger lager.Logger, job string) (db.Lock, bool, error)
}

//go:generate counterfeiter . Scanner

type Scanner interface {
	Scan(lager.Logger, string) error
}

func (s *Scheduler) Schedule(
	logger lager.Logger,
	versions *algorithm.VersionsDB,
	jobConfig atc.JobConfig,
	resourceConfigs atc.ResourceConfigs,
	resourceTypes atc.ResourceTypes,
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

	return s.BuildStarter.TryStartAllPendingBuilds(logger, jobConfig, resourceConfigs, resourceTypes)
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

	lock, acquired, err := s.DB.AcquireResourceCheckingForJobLock(
		logger,
		jobConfig.Name,
	)
	if err != nil {
		logger.Error("failed-to-lock-resource-checking-job", err)
		return nil, nil, err
	}

	build, err := s.DB.CreateJobBuild(jobConfig.Name)
	if err != nil {
		logger.Error("failed-to-create-job-build", err)
		if acquired {
			lock.Release()
		}
		return nil, nil, err
	}

	wg := new(sync.WaitGroup)
	wg.Add(1)

	go func() {
		defer wg.Done()

		if acquired {
			defer lock.Release()

			jobBuildInputs := config.JobInputs(jobConfig)
			for _, input := range jobBuildInputs {
				scanLog := logger.Session("scan", lager.Data{
					"input":    input.Name,
					"resource": input.Resource,
				})

				err := s.Scanner.Scan(scanLog, input.Resource)
				if err != nil {
					return
				}
			}

			versions, err := s.DB.LoadVersionsDB()
			if err != nil {
				logger.Error("failed-to-load-versions-db", err)
				return
			}

			_, err = s.InputMapper.SaveNextInputMapping(logger, versions, jobConfig)
			if err != nil {
				return
			}

			lock.Release()
		}

		err = s.BuildStarter.TryStartAllPendingBuilds(logger, jobConfig, resourceConfigs, resourceTypes)
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
