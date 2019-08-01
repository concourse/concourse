package scheduler

import (
	"code.cloudfoundry.org/lager"
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db"
)

//go:generate counterfeiter . BuildStarter

type BuildStarter interface {
	TryStartPendingBuildsForJob(
		logger lager.Logger,
		job db.Job,
		resources db.Resources,
		resourceTypes atc.VersionedResourceTypes,
	) error
}

//go:generate counterfeiter . BuildFactory

type BuildFactory interface {
	Create(atc.JobConfig, atc.ResourceConfigs, atc.VersionedResourceTypes, []db.BuildInput) (atc.Plan, error)
}

func NewBuildStarter(
	pipeline db.Pipeline,
	factory BuildFactory,
	algorithm Algorithm,
) BuildStarter {
	return &buildStarter{
		pipeline:  pipeline,
		factory:   factory,
		algorithm: algorithm,
	}
}

type buildStarter struct {
	pipeline  db.Pipeline
	factory   BuildFactory
	algorithm Algorithm
}

func (s *buildStarter) TryStartPendingBuildsForJob(
	logger lager.Logger,
	job db.Job,
	resources db.Resources,
	resourceTypes atc.VersionedResourceTypes,
) error {
	nextPendingBuilds, err := job.GetPendingBuilds()
	if err != nil {
		logger.Error("failed-to-get-all-next-pending-builds", err)
		return err
	}

	for _, nextPendingBuild := range nextPendingBuilds {
		started, err := s.tryStartNextPendingBuild(logger, nextPendingBuild, job, resources, resourceTypes)
		if err != nil {
			return err
		}

		if !started {
			break // stop scheduling next builds after failing to schedule a build
		}
	}

	return nil
}

func (s *buildStarter) tryStartNextPendingBuild(
	logger lager.Logger,
	nextPendingBuild db.Build,
	job db.Job,
	resources db.Resources,
	resourceTypes atc.VersionedResourceTypes,
) (bool, error) {
	logger = logger.Session("try-start-next-pending-build", lager.Data{
		"build-id":   nextPendingBuild.ID(),
		"build-name": nextPendingBuild.Name(),
	})

	pipelinePaused, err := s.pipeline.CheckPaused()
	if err != nil {
		logger.Error("failed-to-check-if-pipeline-is-paused", err)
		return false, err
	}
	if pipelinePaused {
		return false, nil
	}

	if job.Paused() {
		return false, nil
	}

	scheduled, err := job.ScheduleBuild(nextPendingBuild)
	if err != nil {
		logger.Error("failed-to-use-inputs", err)
		return false, err
	}

	if !scheduled {
		logger.Debug("build-not-scheduled")
		return false, nil
	}

	if nextPendingBuild.IsManuallyTriggered() {
		for _, input := range job.Config().Inputs() {
			resource, found := resources.Lookup(input.Resource)

			if !found {
				logger.Debug("failed-to-find-resource")
				return false, nil
			}

			if resource.CurrentPinnedVersion() != nil {
				continue
			}

			if resource.LastCheckEndTime().Before(nextPendingBuild.CreateTime()) {
				return false, nil
			}
		}

		versions, err := s.pipeline.LoadVersionsDB()
		if err != nil {
			logger.Error("failed-to-load-versions-db", err)
			return false, err
		}

		inputMapping, resolved, err := s.algorithm.Compute(versions, job, resources)
		if err != nil {
			return false, err
		}

		err = job.SaveNextInputMapping(inputMapping, resolved)
		if err != nil {
			logger.Error("failed-to-save-next-input-mapping", err)
			return false, err
		}

		dbResourceTypes, err := s.pipeline.ResourceTypes()
		if err != nil {
			return false, err
		}
		resourceTypes = dbResourceTypes.Deserialize()
	}

	buildInputs, found, err := nextPendingBuild.AdoptInputsAndPipes()
	if err != nil {
		logger.Error("failed-to-adopt-build-pipes", err)
		return false, err
	}

	if !found {
		return false, nil
	}

	resourceConfigs := atc.ResourceConfigs{}
	for _, v := range resources {
		resourceConfigs = append(resourceConfigs, atc.ResourceConfig{
			Name:   v.Name(),
			Type:   v.Type(),
			Source: v.Source(),
			Tags:   v.Tags(),
		})
	}

	plan, err := s.factory.Create(job.Config(), resourceConfigs, resourceTypes, buildInputs)
	if err != nil {
		// Don't use ErrorBuild because it logs a build event, and this build hasn't started
		if err = nextPendingBuild.Finish(db.BuildStatusErrored); err != nil {
			logger.Error("failed-to-mark-build-as-errored", err)
		}
		return false, nil
	}

	started, err := nextPendingBuild.Start(plan)
	if err != nil {
		logger.Error("failed-to-mark-build-as-started", err)
		return false, nil
	}

	if !started {
		if err = nextPendingBuild.Finish(db.BuildStatusAborted); err != nil {
			logger.Error("failed-to-mark-build-as-finished", err)
		}
		return false, nil
	}

	return true, nil
}
