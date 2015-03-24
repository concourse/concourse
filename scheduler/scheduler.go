package scheduler

import (
	"errors"
	"sync"

	"github.com/pivotal-golang/lager"

	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/engine"
)

var ErrPredeterminedInputsDifferFromConfiguration = errors.New("predetermined build inputs out of sync with configuration")

//go:generate counterfeiter . SchedulerDB

type SchedulerDB interface {
	ScheduleBuild(buildID int, serial bool) (bool, error)
	ErrorBuild(buildID int, err error) error

	SaveResourceVersions(atc.ResourceConfig, []atc.Version) error
	GetLatestInputVersions([]atc.JobInput) ([]db.BuildInput, error)

	CreateJobBuild(job string) (db.Build, error)

	GetJobBuildForInputs(job string, inputs []db.BuildInput) (db.Build, error)
	CreateJobBuildWithInputs(job string, inputs []db.BuildInput) (db.Build, error)

	GetNextPendingBuild(job string) (db.Build, []db.BuildInput, error)

	GetAllStartedBuilds() ([]db.Build, error)
}

//go:generate counterfeiter . BuildFactory

type BuildFactory interface {
	Create(atc.JobConfig, atc.ResourceConfigs, []db.BuildInput) (atc.Plan, error)
}

type Waiter interface {
	Wait()
}

//go:generate counterfeiter . Scanner

type Scanner interface {
	Scan(lager.Logger, string) error
}

type Scheduler struct {
	DB      SchedulerDB
	Factory BuildFactory
	Engine  engine.Engine
	Scanner Scanner
}

func (s *Scheduler) BuildLatestInputs(logger lager.Logger, job atc.JobConfig, resources atc.ResourceConfigs) error {
	logger = logger.Session("build-latest")

	inputs := job.Inputs()

	if len(inputs) == 0 {
		// no inputs; no-op
		return nil
	}

	latestInputs, err := s.DB.GetLatestInputVersions(inputs)
	if err != nil {
		if err == db.ErrNoVersions {
			logger.Debug("no-input-versions-available")
			return nil
		}

		logger.Error("failed-to-get-latest-input-versions", err)
		return err
	}

	checkInputs := []db.BuildInput{}
	for _, input := range latestInputs {
		for _, ji := range inputs {
			if ji.Name == input.Name {
				if ji.Trigger {
					checkInputs = append(checkInputs, input)
				}

				break
			}
		}
	}

	if len(checkInputs) == 0 {
		logger.Debug("no-triggered-input-versions")
		return nil
	}

	existingBuild, err := s.DB.GetJobBuildForInputs(job.Name, checkInputs)
	if err == nil {
		logger.Debug("build-already-exists-for-inputs", lager.Data{
			"existing-build": existingBuild.ID,
		})

		return nil
	}

	build, err := s.DB.CreateJobBuildWithInputs(job.Name, latestInputs)
	if err != nil {
		logger.Error("failed-to-create-build", err)
		return err
	}

	logger = logger.WithData(lager.Data{"build": build.ID})

	logger.Debug("created-build")

	scheduled, err := s.DB.ScheduleBuild(build.ID, job.Serial)
	if err != nil {
		logger.Error("failed-to-scheduled-build", err)
		return err
	}

	if !scheduled {
		logger.Debug("build-could-not-be-scheduled")
		return nil
	}

	plan, err := s.Factory.Create(job, resources, latestInputs)
	if err != nil {
		logger.Error("failed-to-create", err)
		return err
	}

	createdBuild, err := s.Engine.CreateBuild(build, plan)
	if err != nil {
		logger.Error("failed-to-build", err)
		return err
	}

	logger.Info("building")

	go createdBuild.Resume(logger)

	return nil
}

func (s *Scheduler) TryNextPendingBuild(logger lager.Logger, job atc.JobConfig, resources atc.ResourceConfigs) Waiter {
	logger = logger.Session("try-next-pending")

	wg := new(sync.WaitGroup)

	wg.Add(1)
	go func() {
		build, inputs, err := s.DB.GetNextPendingBuild(job.Name)
		if err != nil {
			if err == db.ErrNoBuild {
				wg.Done()

				return
			}

			logger.Error("failed-to-get-next-pending-build", err)

			wg.Done()

			return
		}

		createdBuild := s.scheduleAndResumePendingBuild(logger, build, inputs, job, resources)

		wg.Done()

		if createdBuild != nil {
			logger.Info("building")
			createdBuild.Resume(logger)
		}
	}()

	return wg
}

func (s *Scheduler) TriggerImmediately(logger lager.Logger, job atc.JobConfig, resources atc.ResourceConfigs) (db.Build, error) {
	logger = logger.Session("trigger-immediately")

	build, err := s.DB.CreateJobBuild(job.Name)
	if err != nil {
		logger.Error("failed-to-create-build", err)
		return db.Build{}, err
	}

	go func() {
		createdBuild := s.scheduleAndResumePendingBuild(logger, build, nil, job, resources)
		if createdBuild != nil {
			logger.Info("building")
			createdBuild.Resume(logger)
		}
	}()

	return build, nil
}

func (s *Scheduler) TrackInFlightBuilds(logger lager.Logger) error {
	builds, err := s.DB.GetAllStartedBuilds()
	if err != nil {
		return err
	}

	for _, b := range builds {
		tLog := logger.Session("track", lager.Data{
			"build": b.ID,
		})

		engineBuild, err := s.Engine.LookupBuild(b)
		if err != nil {
			tLog.Error("failed-to-lookup-build", err)

			err := s.DB.ErrorBuild(b.ID, err)
			if err != nil {
				tLog.Error("failed-to-mark-build-as-errored", err)
			}

			continue
		}

		go engineBuild.Resume(tLog)
	}

	return nil
}

func (s *Scheduler) scheduleAndResumePendingBuild(logger lager.Logger, build db.Build, inputs []db.BuildInput, job atc.JobConfig, resources atc.ResourceConfigs) engine.Build {
	logger = logger.WithData(lager.Data{"build": build.ID})

	scheduled, err := s.DB.ScheduleBuild(build.ID, job.Serial)
	if err != nil {
		logger.Error("failed-to-schedule-build", err)
		return nil
	}

	if !scheduled {
		logger.Debug("build-could-not-be-scheduled")
		return nil
	}

	buildInputs := job.Inputs()

	if len(inputs) == 0 {
		for _, input := range buildInputs {
			scanLog := logger.Session("scan", lager.Data{
				"input":    input.Name,
				"resource": input.Resource,
			})

			err := s.Scanner.Scan(scanLog, input.Resource)
			if err != nil {
				scanLog.Error("failed-to-scan", err)

				err := s.DB.ErrorBuild(build.ID, err)
				if err != nil {
					logger.Error("failed-to-mark-build-as-errored", err)
				}

				return nil
			}

			scanLog.Info("done")
		}

		inputs, err = s.DB.GetLatestInputVersions(buildInputs)
		if err != nil {
			logger.Error("failed-to-get-latest-input-versions", err)
			return nil
		}
	} else if len(inputs) != len(buildInputs) {
		logger.Error("input-configuration-mismatch", nil, lager.Data{
			"build-inputs": inputs,
			"job-inputs":   buildInputs,
		})

		err := s.DB.ErrorBuild(build.ID, ErrPredeterminedInputsDifferFromConfiguration)
		if err != nil {
			logger.Error("failed-to-mark-build-as-errored", err)
		}

		return nil
	}

	plan, err := s.Factory.Create(job, resources, inputs)
	if err != nil {
		logger.Error("failed-to-create-build-plan", err)
		return nil
	}

	createdBuild, err := s.Engine.CreateBuild(build, plan)
	if err != nil {
		logger.Error("failed-to-create-build", err)
		return nil
	}

	return createdBuild
}
