package scheduler

import (
	"sync"
	"time"

	"github.com/pivotal-golang/lager"

	"github.com/concourse/atc"
	"github.com/concourse/atc/config"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/db/algorithm"
	"github.com/concourse/atc/engine"
)

//go:generate counterfeiter . PipelineDB

type PipelineDB interface {
	CreateJobBuild(job string) (db.Build, error)
	CreateJobBuildForCandidateInputs(job string) (db.Build, bool, error)
	ScheduleBuild(buildID int, jobConfig atc.JobConfig) (bool, error)

	GetJobBuildForInputs(job string, inputs []db.BuildInput) (db.Build, error)
	GetNextPendingBuild(job string) (db.Build, error)

	LoadVersionsDB() (*algorithm.VersionsDB, error)
	GetLatestInputVersions(versions *algorithm.VersionsDB, job string, inputs []config.JobInput) ([]db.BuildInput, error)
	SaveResourceVersions(atc.ResourceConfig, []atc.Version) error
	UseInputsForBuild(buildID int, inputs []db.BuildInput) error
}

//go:generate counterfeiter . BuildsDB

type BuildsDB interface {
	GetAllStartedBuilds() ([]db.Build, error)
	ErrorBuild(buildID int, err error) error
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
	PipelineDB PipelineDB
	BuildsDB   BuildsDB
	Factory    BuildFactory
	Engine     engine.Engine
	Scanner    Scanner
}

func (s *Scheduler) BuildLatestInputs(logger lager.Logger, versions *algorithm.VersionsDB, job atc.JobConfig, resources atc.ResourceConfigs) error {
	logger = logger.Session("build-latest")

	inputs := config.JobInputs(job)

	if len(inputs) == 0 {
		// no inputs; no-op
		return nil
	}

	latestInputs, err := s.PipelineDB.GetLatestInputVersions(versions, job.Name, inputs)
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

	existingBuild, err := s.PipelineDB.GetJobBuildForInputs(job.Name, checkInputs)
	if err == nil {
		logger.Debug("build-already-exists-for-inputs", lager.Data{
			"existing-build": existingBuild.ID,
		})

		return nil
	}

	build, created, err := s.PipelineDB.CreateJobBuildForCandidateInputs(job.Name)
	if err != nil {
		logger.Error("failed-to-create-build", err)
		return err
	}

	if !created {
		logger.Debug("waiting-for-existing-build-to-determine-inputs", lager.Data{
			"existing-build": build.ID,
		})
		return nil
	}

	logger.Debug("created-build", lager.Data{"build": build.ID})

	// NOTE: this is intentionally serial within a scheduler tick, so that
	// multiple ATCs don't do redundant work to determine a build's inputs.

	s.scheduleAndResumePendingBuild(logger, versions, build, job, resources)

	return nil
}

func (s *Scheduler) TryNextPendingBuild(logger lager.Logger, versions *algorithm.VersionsDB, job atc.JobConfig, resources atc.ResourceConfigs) Waiter {
	logger = logger.Session("try-next-pending")

	wg := new(sync.WaitGroup)

	wg.Add(1)
	go func() {
		defer wg.Done()

		build, err := s.PipelineDB.GetNextPendingBuild(job.Name)
		if err != nil {
			if err == db.ErrNoBuild {
				return
			}

			logger.Error("failed-to-get-next-pending-build", err)

			return
		}

		s.scheduleAndResumePendingBuild(logger, versions, build, job, resources)
	}()

	return wg
}

func (s *Scheduler) TriggerImmediately(logger lager.Logger, job atc.JobConfig, resources atc.ResourceConfigs) (db.Build, Waiter, error) {
	logger = logger.Session("trigger-immediately")

	build, err := s.PipelineDB.CreateJobBuild(job.Name)
	if err != nil {
		logger.Error("failed-to-create-build", err)
		return db.Build{}, nil, err
	}

	wg := new(sync.WaitGroup)
	wg.Add(1)

	// do not block request on scanning input versions
	go func() {
		defer wg.Done()
		s.scheduleAndResumePendingBuild(logger, nil, build, job, resources)
	}()

	return build, wg, nil
}

func (s *Scheduler) scheduleAndResumePendingBuild(logger lager.Logger, versions *algorithm.VersionsDB, build db.Build, job atc.JobConfig, resources atc.ResourceConfigs) engine.Build {
	logger = logger.WithData(lager.Data{"build": build.ID})

	scheduled, err := s.PipelineDB.ScheduleBuild(build.ID, job)
	if err != nil {
		logger.Error("failed-to-schedule-build", err)
		return nil
	}

	if !scheduled {
		logger.Debug("build-could-not-be-scheduled")
		return nil
	}

	buildInputs := config.JobInputs(job)

	if versions == nil {
		for _, input := range buildInputs {
			scanLog := logger.Session("scan", lager.Data{
				"input":    input.Name,
				"resource": input.Resource,
			})

			err := s.Scanner.Scan(scanLog, input.Resource)
			if err != nil {
				scanLog.Error("failed-to-scan", err)

				err := s.BuildsDB.ErrorBuild(build.ID, err)
				if err != nil {
					logger.Error("failed-to-mark-build-as-errored", err)
				}

				return nil
			}

			scanLog.Info("done")
		}

		loadStart := time.Now()

		vLog := logger.Session("loading-versions")
		vLog.Info("start")

		versions, err = s.PipelineDB.LoadVersionsDB()
		if err != nil {
			vLog.Error("failed", err)
			return nil
		}

		vLog.Info("done", lager.Data{"took": time.Since(loadStart).String()})
	}

	inputs, err := s.PipelineDB.GetLatestInputVersions(versions, job.Name, buildInputs)
	if err != nil {
		logger.Error("failed-to-get-latest-input-versions", err)
		return nil
	}

	err = s.PipelineDB.UseInputsForBuild(build.ID, inputs)
	if err != nil {
		logger.Error("failed-to-use-inputs-for-build", err)
		return nil
	}

	plan, err := s.Factory.Create(job, resources, inputs)
	if err != nil {
		logger.Error("failed-to-create-build-plan", err)
		s.BuildsDB.ErrorBuild(build.ID, err)
		return nil
	}

	createdBuild, err := s.Engine.CreateBuild(build, plan)
	if err != nil {
		logger.Error("failed-to-create-build", err)
		return nil
	}

	if createdBuild != nil {
		logger.Info("building")
		go createdBuild.Resume(logger)
	}

	return createdBuild
}
