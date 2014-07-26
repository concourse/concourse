package scheduler

import (
	"github.com/concourse/atc/builder"
	"github.com/concourse/atc/builds"
	"github.com/concourse/atc/config"
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
}

func (s *Scheduler) BuildLatestInputs(job config.Job) error {
	inputs, err := s.DB.GetLatestInputVersions(job.Inputs)
	if err != nil {
		return err
	}

	_, err = s.DB.GetBuildForInputs(job.Name, inputs)
	if err == nil {
		return nil
	}

	build, err := s.DB.CreateBuildWithInputs(job.Name, inputs)
	if err != nil {
		return err
	}

	return s.Builder.Build(build, job, inputs)
}

func (s *Scheduler) TryNextPendingBuild(job config.Job) error {
	build, inputs, err := s.DB.GetNextPendingBuild(job.Name)
	if err != nil {
		return err
	}

	return s.Builder.Build(build, job, inputs)
}
