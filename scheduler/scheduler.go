package scheduler

import (
	tbuilds "github.com/concourse/turbine/api/builds"
	"github.com/pivotal-golang/lager"

	"github.com/concourse/atc/builder"
	"github.com/concourse/atc/builds"
	"github.com/concourse/atc/config"
)

type SchedulerDB interface {
	ScheduleBuild(job string, build string, serial bool) (bool, error)

	CreateBuildWithInputs(job string, inputs builds.VersionedResources) (builds.Build, error)
	GetLatestInputVersions([]config.Input) (builds.VersionedResources, error)
	GetJobBuildForInputs(job string, inputs builds.VersionedResources) (builds.Build, error)

	GetNextPendingBuild(job string) (builds.Build, builds.VersionedResources, error)
}

type BuildFactory interface {
	Create(config.Job, builds.VersionedResources) (tbuilds.Build, error)
}

type Scheduler struct {
	Logger  lager.Logger
	DB      SchedulerDB
	Factory BuildFactory
	Builder builder.Builder
}

func (s *Scheduler) BuildLatestInputs(job config.Job) error {
	buildLog := s.Logger.Session("build-latest")

	inputs, err := s.DB.GetLatestInputVersions(job.Inputs)
	if err != nil {
		buildLog.Error("failed-to-get-latest-input-versions", err)
		return err
	}

	checkInputs := builds.VersionedResources{}
	for _, input := range job.Inputs {
		if input.DontCheck {
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

	_, err = s.DB.GetJobBuildForInputs(job.Name, checkInputs)
	if err == nil {
		return nil
	}

	build, err := s.DB.CreateBuildWithInputs(job.Name, inputs)
	if err != nil {
		buildLog.Error("failed-to-create-build", err, lager.Data{
			"inputs": inputs,
		})
		return err
	}

	scheduled, err := s.DB.ScheduleBuild(job.Name, build.Name, job.Serial)
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

	scheduled, err := s.DB.ScheduleBuild(job.Name, build.Name, job.Serial)
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

func (s *Scheduler) TriggerImmediately(job config.Job) (builds.Build, error) {
	buildLog := s.Logger.Session("trigger-immediately")

	passedInputs := []config.Input{}
	for _, input := range job.Inputs {
		if len(input.Passed) == 0 {
			continue
		}

		passedInputs = append(passedInputs, input)
	}

	var inputs builds.VersionedResources
	var err error

	if len(passedInputs) > 0 {
		inputs, err = s.DB.GetLatestInputVersions(passedInputs)
		if err != nil {
			buildLog.Error("failed-to-get-build-inputs", err)
			return builds.Build{}, err
		}
	}

	build, err := s.DB.CreateBuildWithInputs(job.Name, inputs)
	if err != nil {
		buildLog.Error("failed-to-create-build", err)
		return builds.Build{}, err
	}

	scheduled, err := s.DB.ScheduleBuild(job.Name, build.Name, job.Serial)
	if err != nil {
		return builds.Build{}, err
	}

	if !scheduled {
		return build, nil
	}

	turbineBuild, err := s.Factory.Create(job, inputs)
	if err != nil {
		buildLog.Error("failed-to-create", err)
		return builds.Build{}, err
	}

	err = s.Builder.Build(build, turbineBuild)
	if err != nil {
		buildLog.Error("failed-to-build", err)
		return builds.Build{}, err
	}

	return build, nil
}
