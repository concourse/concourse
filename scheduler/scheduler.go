package scheduler

import (
	"fmt"

	"github.com/pivotal-golang/lager"

	"github.com/concourse/atc/builder"
	"github.com/concourse/atc/config"
	"github.com/concourse/atc/db"
	"github.com/concourse/turbine"
)

type SchedulerLocker interface {
	AcquireLock(names ...string) (db.Lock, error)
}

type SchedulerDB interface {
	ScheduleBuild(buildID int, serial bool) (bool, error)

	GetLatestInputVersions([]config.Input) (db.VersionedResources, error)
	CreateJobBuildWithInputs(job string, inputs db.VersionedResources) (db.Build, error)
	GetJobBuildForInputs(job string, inputs db.VersionedResources) (db.Build, error)

	GetNextPendingBuild(job string) (db.Build, db.VersionedResources, error)

	GetAllStartedBuilds() ([]db.Build, error)
}

type BuildFactory interface {
	Create(config.Job, db.VersionedResources) (turbine.Build, error)
}

type BuildTracker interface {
	TrackBuild(db.Build) error
}

type Scheduler struct {
	Logger  lager.Logger
	Locker  SchedulerLocker
	DB      SchedulerDB
	Factory BuildFactory
	Builder builder.Builder
	Tracker BuildTracker
}

func (s *Scheduler) BuildLatestInputs(job config.Job) error {
	if len(job.Inputs) == 0 {
		// no inputs; no-op
		return nil
	}

	buildLog := s.Logger.Session("build-latest")

	inputs, err := s.DB.GetLatestInputVersions(job.Inputs)
	if err != nil {
		buildLog.Error("failed-to-get-latest-input-versions", err)
		return err
	}

	checkInputs := db.VersionedResources{}
	for _, input := range job.Inputs {
		if input.Trigger != nil && !*input.Trigger {
			continue
		}

		vr, found := inputs.Lookup(input.Resource)
		if !found {
			// this really shouldn't happen, but...
			buildLog.Error("failed-to-find-version", nil, lager.Data{
				"resource": input.Resource,
				"inputs":   inputs,
			})
			continue
		}

		checkInputs = append(checkInputs, vr)
	}

	if len(checkInputs) == 0 {
		return nil
	}

	lockNames := []string{}
	for _, input := range checkInputs {
		lockNames = append(lockNames, fmt.Sprintf("resource: %s", input.Name))
	}

	lock, err := s.Locker.AcquireLock(lockNames...)
	if err != nil {
		buildLog.Error("failed-to-acquire-inputs-lock", err, lager.Data{
			"inputs": inputs,
		})
		return err
	}
	_, err = s.DB.GetJobBuildForInputs(job.Name, checkInputs)
	lock.Release()
	if err == nil {
		return nil
	}

	build, err := s.DB.CreateJobBuildWithInputs(job.Name, inputs)
	if err != nil {
		buildLog.Error("failed-to-create-build", err, lager.Data{
			"inputs": inputs,
		})
		return err
	}

	scheduled, err := s.DB.ScheduleBuild(build.ID, job.Serial)
	if err != nil {
		return err
	}

	if !scheduled {
		return nil
	}

	buildLog.Info("building", lager.Data{
		"build":  build,
		"inputs": inputs,
	})

	turbineBuild, err := s.Factory.Create(job, inputs)
	if err != nil {
		buildLog.Error("failed-to-create", err)
		return err
	}

	err = s.Builder.Build(build, turbineBuild)
	if err != nil {
		buildLog.Error("failed-to-build", err)
		return err
	}

	return nil
}

func (s *Scheduler) TryNextPendingBuild(job config.Job) error {
	buildLog := s.Logger.Session("trigger-pending")

	build, inputs, err := s.DB.GetNextPendingBuild(job.Name)
	if err != nil {
		return err
	}

	scheduled, err := s.DB.ScheduleBuild(build.ID, job.Serial)
	if err != nil {
		return err
	}

	if !scheduled {
		return nil
	}

	turbineBuild, err := s.Factory.Create(job, inputs)
	if err != nil {
		buildLog.Error("failed-to-create", err)
		return err
	}

	err = s.Builder.Build(build, turbineBuild)
	if err != nil {
		buildLog.Error("failed-to-build", err)
		return err
	}

	return nil
}

func (s *Scheduler) TriggerImmediately(job config.Job) (db.Build, error) {
	buildLog := s.Logger.Session("trigger-immediately")

	passedInputs := []config.Input{}
	for _, input := range job.Inputs {
		if len(input.Passed) == 0 {
			continue
		}

		passedInputs = append(passedInputs, input)
	}

	var inputs db.VersionedResources
	var err error

	if len(passedInputs) > 0 {
		inputs, err = s.DB.GetLatestInputVersions(passedInputs)
		if err != nil {
			buildLog.Error("failed-to-get-build-inputs", err)
			return db.Build{}, err
		}
	}

	build, err := s.DB.CreateJobBuildWithInputs(job.Name, inputs)
	if err != nil {
		buildLog.Error("failed-to-create-build", err)
		return db.Build{}, err
	}

	scheduled, err := s.DB.ScheduleBuild(build.ID, job.Serial)
	if err != nil {
		return db.Build{}, err
	}

	if !scheduled {
		return build, nil
	}

	turbineBuild, err := s.Factory.Create(job, inputs)
	if err != nil {
		buildLog.Error("failed-to-create", err)
		return db.Build{}, err
	}

	err = s.Builder.Build(build, turbineBuild)
	if err != nil {
		buildLog.Error("failed-to-build", err)
		return db.Build{}, err
	}

	return build, nil
}

func (s *Scheduler) TrackInFlightBuilds() error {
	builds, err := s.DB.GetAllStartedBuilds()
	if err != nil {
		return err
	}

	for _, b := range builds {
		go s.Tracker.TrackBuild(b)
	}

	return nil
}
