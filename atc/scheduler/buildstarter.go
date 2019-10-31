package scheduler

import (
	"code.cloudfoundry.org/lager"
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/scheduler/algorithm"
)

//go:generate counterfeiter . BuildStarter

type BuildStarter interface {
	TryStartPendingBuildsForJob(
		logger lager.Logger,
		pipeline db.Pipeline,
		job db.Job,
		resources db.Resources,
		relatedJobs algorithm.NameToIDMap,
	) error
}

//go:generate counterfeiter . BuildFactory

type BuildFactory interface {
	Create(atc.JobConfig, atc.ResourceConfigs, atc.VersionedResourceTypes, []db.BuildInput) (atc.Plan, error)
}

type Build interface {
	db.Build

	BuildInputs(lager.Logger) ([]db.BuildInput, bool, error)
}

func NewBuildStarter(
	factory BuildFactory,
	algorithm Algorithm,
) BuildStarter {
	return &buildStarter{
		factory:   factory,
		algorithm: algorithm,
	}
}

type buildStarter struct {
	factory   BuildFactory
	algorithm Algorithm
}

func (s *buildStarter) TryStartPendingBuildsForJob(
	logger lager.Logger,
	pipeline db.Pipeline,
	job db.Job,
	resources db.Resources,
	relatedJobs algorithm.NameToIDMap,
) error {
	nextPendingBuilds, err := job.GetPendingBuilds()
	if err != nil {
		logger.Error("failed-to-get-all-next-pending-builds", err)
		return err
	}

	schedulableBuilds := s.constructBuilds(pipeline, job, resources, relatedJobs, nextPendingBuilds)

	for _, nextSchedulableBuild := range schedulableBuilds {
		started, err := s.tryStartNextPendingBuild(logger, pipeline, nextSchedulableBuild, job, resources)
		if err != nil {
			return err
		}

		if !started {
			break // stop scheduling next builds after failing to schedule a build
		}
	}

	return nil
}

func (s *buildStarter) constructBuilds(pipeline db.Pipeline, job db.Job, resources db.Resources, relatedJobIDs map[string]int, builds []db.Build) []Build {
	schedulableBuilds := []Build{}

	for _, nextPendingBuild := range builds {
		if nextPendingBuild.IsManuallyTriggered() {
			schedulableBuilds = append(schedulableBuilds, &manualTriggerBuild{
				Build:         nextPendingBuild,
				algorithm:     s.algorithm,
				pipeline:      pipeline,
				job:           job,
				resources:     resources,
				relatedJobIDs: relatedJobIDs,
			})
		} else if nextPendingBuild.RerunOf() != 0 {
			schedulableBuilds = append(schedulableBuilds, &rerunBuild{
				Build: nextPendingBuild,
			})
		} else {
			schedulableBuilds = append(schedulableBuilds, &schedulerBuild{
				Build: nextPendingBuild,
			})
		}
	}

	return schedulableBuilds
}

func (s *buildStarter) tryStartNextPendingBuild(
	logger lager.Logger,
	pipeline db.Pipeline,
	nextPendingBuild Build,
	job db.Job,
	resources db.Resources,
) (bool, error) {
	logger = logger.Session("try-start-next-pending-build", lager.Data{
		"build-id":   nextPendingBuild.ID(),
		"build-name": nextPendingBuild.Name(),
	})

	if nextPendingBuild.IsAborted() {
		logger.Debug("cancel-aborted-pending-build")
		err := nextPendingBuild.Finish(db.BuildStatusAborted)
		if err != nil {
			return false, err
		}

		return true, nil
	}

	pipelinePaused, err := pipeline.CheckPaused()
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

	buildInputs, found, err := nextPendingBuild.BuildInputs(logger)
	if err != nil {
		logger.Error("failed-to-adopt-build-pipes", err)
		return false, err
	}

	if !found {
		return false, nil
	}

	dbResourceTypes, err := pipeline.ResourceTypes()
	if err != nil {
		return false, err
	}
	resourceTypes := dbResourceTypes.Deserialize()

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
