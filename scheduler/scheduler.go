package scheduler

import (
	"github.com/pivotal-golang/lager"

	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/engine"
)

//go:generate counterfeiter . SchedulerDB

type SchedulerDB interface {
	ScheduleBuild(buildID int, serial bool) (bool, error)
	FinishBuild(buildID int, status db.Status) error

	GetLatestInputVersions([]atc.JobInputConfig) (db.SavedVersionedResources, error)

	GetJobBuildForInputs(job string, inputs []db.BuildInput) (db.Build, error)
	CreateJobBuildWithInputs(job string, inputs []db.BuildInput) (db.Build, error)

	GetNextPendingBuild(job string) (db.Build, []db.BuildInput, error)

	GetAllStartedBuilds() ([]db.Build, error)
}

//go:generate counterfeiter . BuildFactory

type BuildFactory interface {
	Create(atc.JobConfig, atc.ResourceConfigs, []db.BuildInput) (atc.BuildPlan, error)
}

type Scheduler struct {
	Locker  Locker
	DB      SchedulerDB
	Factory BuildFactory
	Engine  engine.Engine
}

func (s *Scheduler) BuildLatestInputs(logger lager.Logger, job atc.JobConfig, resources atc.ResourceConfigs) error {
	logger = logger.Session("build-latest")

	if len(job.Inputs) == 0 {
		// no inputs; no-op
		return nil
	}

	lock, err := s.lockVersionUpdatesFor(job.Inputs)
	if err != nil {
		logger.Error("failed-to-acquire-inputs-lock", err)
		return err
	}

	versions, err := s.DB.GetLatestInputVersions(job.Inputs)

	lock.Release()

	if err != nil {
		if err == db.ErrNoVersions {
			logger.Debug("no-input-versions-available")
			return nil
		}

		logger.Error("failed-to-get-latest-input-versions", err)
		return err
	}

	inputs := []db.BuildInput{}
	for _, input := range job.Inputs {
		if !input.Trigger() {
			continue
		}

		vr, found := versions.Lookup(input.Resource)
		if !found {
			// this really shouldn't happen, but...
			logger.Error("failed-to-find-version", nil, lager.Data{
				"resource": input.Resource,
				"versions": versions,
			})
			continue
		}

		inputs = append(inputs, db.BuildInput{
			Name:              input.Name(),
			VersionedResource: vr.VersionedResource,
		})
	}

	if len(inputs) == 0 {
		logger.Debug("no-triggered-input-versions")
		return nil
	}

	existingBuild, err := s.DB.GetJobBuildForInputs(job.Name, inputs)
	if err == nil {
		logger.Debug("build-already-exists-for-inputs", lager.Data{
			"existing-build": existingBuild.ID,
		})

		return nil
	}

	build, err := s.DB.CreateJobBuildWithInputs(job.Name, inputs)
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

	buildPlan, err := s.Factory.Create(job, resources, inputs)
	if err != nil {
		logger.Error("failed-to-create", err)
		return err
	}

	createdBuild, err := s.Engine.CreateBuild(build, buildPlan)
	if err != nil {
		logger.Error("failed-to-build", err)
		return err
	}

	logger.Info("building")

	go createdBuild.Resume(logger)

	return nil
}

func (s *Scheduler) TryNextPendingBuild(logger lager.Logger, job atc.JobConfig, resources atc.ResourceConfigs) error {
	logger = logger.Session("try-next-pending")

	build, inputs, err := s.DB.GetNextPendingBuild(job.Name)
	if err != nil {
		if err == db.ErrNoBuild {
			return nil
		}

		return err
	}

	logger = logger.WithData(lager.Data{"build": build.ID})

	scheduled, err := s.DB.ScheduleBuild(build.ID, job.Serial)
	if err != nil {
		logger.Error("failed-to-schedule-build", err)
		return err
	}

	if !scheduled {
		logger.Debug("build-could-not-be-scheduled")
		return nil
	}

	buildPlan, err := s.Factory.Create(job, resources, inputs)
	if err != nil {
		logger.Error("failed-to-create-build-plan", err)
		return err
	}

	createdBuild, err := s.Engine.CreateBuild(build, buildPlan)
	if err != nil {
		logger.Error("failed-to-create-build", err)
		return err
	}

	logger.Info("building")

	go createdBuild.Resume(logger)

	return nil
}

func (s *Scheduler) TriggerImmediately(logger lager.Logger, job atc.JobConfig, resources atc.ResourceConfigs) (db.Build, error) {
	logger = logger.Session("trigger-immediately")

	passedInputs := []atc.JobInputConfig{}
	for _, input := range job.Inputs {
		if len(input.Passed) == 0 {
			continue
		}

		passedInputs = append(passedInputs, input)
	}

	var inputs []db.BuildInput
	var err error

	if len(passedInputs) > 0 {
		versions, err := s.DB.GetLatestInputVersions(passedInputs)
		if err != nil {
			logger.Error("failed-to-get-build-inputs", err)
			return db.Build{}, err
		}

		for _, input := range job.Inputs {
			vr, found := versions.Lookup(input.Resource)
			if found {
				inputs = append(inputs, db.BuildInput{
					Name:              input.Name(),
					VersionedResource: vr.VersionedResource,
				})
			}
		}
	}

	build, err := s.DB.CreateJobBuildWithInputs(job.Name, inputs)
	if err != nil {
		logger.Error("failed-to-create-build", err)
		return db.Build{}, err
	}

	scheduled, err := s.DB.ScheduleBuild(build.ID, job.Serial)
	if err != nil {
		logger.Error("failed-to-schedule-build", err)
		return db.Build{}, err
	}

	if !scheduled {
		logger.Debug("build-could-not-be-scheduled")
		return build, nil
	}

	logger = logger.WithData(lager.Data{"build": build.ID})

	buildPlan, err := s.Factory.Create(job, resources, inputs)
	if err != nil {
		logger.Error("failed-to-create", err)
		return db.Build{}, err
	}

	createdBuild, err := s.Engine.CreateBuild(build, buildPlan)
	if err != nil {
		logger.Error("failed-to-build", err)
		return db.Build{}, err
	}

	logger.Info("building")

	go createdBuild.Resume(logger)

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

			err := s.DB.FinishBuild(b.ID, db.StatusErrored)
			if err != nil {
				tLog.Error("failed-to-mark-build-as-errored", err)
			}

			continue
		}

		go engineBuild.Resume(tLog)
	}

	return nil
}

func (s *Scheduler) lockVersionUpdatesFor(inputs []atc.JobInputConfig) (db.Lock, error) {
	locks := []db.NamedLock{}
	for _, input := range inputs {
		locks = append(locks, db.ResourceLock(input.Resource))
	}

	return s.Locker.AcquireReadLock(locks)
}
