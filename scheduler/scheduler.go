package scheduler

import (
	"github.com/concourse/atc/builder"
	"github.com/concourse/atc/builds"
	"github.com/concourse/atc/config"
	"github.com/pivotal-golang/lager"
)

type SchedulerDB interface {
	CreateBuildWithInputs(job string, inputs builds.VersionedResources) (builds.Build, error)
	GetLatestInputVersions([]config.Input) (builds.VersionedResources, error)
	GetBuildForInputs(job string, inputs builds.VersionedResources) (builds.Build, error)

	GetNextPendingBuild(job string) (builds.Build, builds.VersionedResources, error)
}

type Scheduler struct {
	DB      SchedulerDB
	Builder builder.Builder
	Logger  lager.Logger
}

func (s *Scheduler) BuildLatestInputs(job config.Job) error {
	buildLog := s.Logger.Session("build-latest")

	inputs, err := s.DB.GetLatestInputVersions(job.Inputs)
	if err != nil {
		buildLog.Error("failed-to-get-latest-input-versions", err)
		return err
	}

	_, err = s.DB.GetBuildForInputs(job.Name, inputs)
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

	buildLog.Info("building", lager.Data{
		"build":  build,
		"inputs": inputs,
	})

	err = s.Builder.Build(build, job, inputs)
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

	err = s.Builder.Build(build, job, inputs)
	if err != nil {
		buildLog.Error("failed-to-build", err)
		return err
	}

	return nil
}
