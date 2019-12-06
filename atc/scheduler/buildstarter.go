package scheduler

import (
	"fmt"

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
	) (bool, error)
}

//go:generate counterfeiter . BuildFactory

type BuildFactory interface {
	Create(atc.JobConfig, atc.ResourceConfigs, atc.VersionedResourceTypes, []db.BuildInput) (atc.Plan, error)
}

type Build interface {
	db.Build

	PrepareInputs(lager.Logger) bool
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
) (bool, error) {
	nextPendingBuilds, err := job.GetPendingBuilds()
	if err != nil {
		return false, fmt.Errorf("get pending builds: %w", err)
	}

	schedulableBuilds := s.constructBuilds(job, resources, relatedJobs, nextPendingBuilds)

	for _, nextSchedulableBuild := range schedulableBuilds {
		results, err := s.tryStartNextPendingBuild(logger, pipeline, nextSchedulableBuild, job, resources)
		if err != nil {
			return false, err
		}

		if results.needsRetry {
			return true, nil
		}

		if !results.started {
			// stop scheduling next builds after failing to schedule a build
			return results.needsRetry, nil
		}
	}

	return false, nil
}

func (s *buildStarter) constructBuilds(job db.Job, resources db.Resources, relatedJobIDs map[string]int, builds []db.Build) []Build {
	schedulableBuilds := []Build{}

	for _, nextPendingBuild := range builds {
		if nextPendingBuild.IsManuallyTriggered() {
			schedulableBuilds = append(schedulableBuilds, &manualTriggerBuild{
				Build:         nextPendingBuild,
				algorithm:     s.algorithm,
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

type startResults struct {
	started    bool
	needsRetry bool
}

func (s *buildStarter) tryStartNextPendingBuild(
	logger lager.Logger,
	pipeline db.Pipeline,
	nextPendingBuild Build,
	job db.Job,
	resources db.Resources,
) (startResults, error) {
	logger = logger.Session("try-start-next-pending-build", lager.Data{
		"build-id":   nextPendingBuild.ID(),
		"build-name": nextPendingBuild.Name(),
	})

	if nextPendingBuild.IsAborted() {
		logger.Debug("cancel-aborted-pending-build")
		err := nextPendingBuild.Finish(db.BuildStatusAborted)
		if err != nil {
			return startResults{}, fmt.Errorf("finish aborted build: %w", err)
		}

		return startResults{
			started:    true,
			needsRetry: false,
		}, nil
	}

	pipelinePaused, err := pipeline.CheckPaused()
	if err != nil {
		return startResults{}, fmt.Errorf("check pipeline paused: %w", err)
	}

	if pipelinePaused {
		logger.Debug("pipeline-paused")
		return startResults{
			started:    false,
			needsRetry: false,
		}, nil
	}

	if job.Paused() {
		logger.Debug("job-paused")
		return startResults{
			started:    false,
			needsRetry: false,
		}, nil
	}

	scheduled, err := job.ScheduleBuild(nextPendingBuild)
	if err != nil {
		return startResults{}, fmt.Errorf("schedule build: %w", err)
	}

	if !scheduled {
		logger.Debug("build-not-scheduled")
		return startResults{
			started:    false,
			needsRetry: true,
		}, nil
	}

	succeeded := nextPendingBuild.PrepareInputs(logger)
	if err != nil {
		return startResults{}, fmt.Errorf("prepare inputs: %w", err)
	}

	if !succeeded {
		return startResults{
			started:    false,
			needsRetry: true,
		}, nil
	}

	buildInputs, found, err := nextPendingBuild.BuildInputs(logger)
	if err != nil {
		return startResults{}, fmt.Errorf("get build inputs: %w", err)
	}

	if !found {
		logger.Debug("build-inputs-not-found")

		// don't retry when build inputs are not found because this is due to the
		// inputs being unsatisfiable
		return startResults{
			started:    false,
			needsRetry: false,
		}, nil
	}

	resourceTypes, err := pipeline.ResourceTypes()
	if err != nil {
		return startResults{}, fmt.Errorf("find resource types: %w", err)
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

	plan, err := s.factory.Create(job.Config(), resourceConfigs, resourceTypes.Deserialize(), buildInputs)
	if err != nil {
		// Don't use ErrorBuild because it logs a build event, and this build hasn't started
		if err = nextPendingBuild.Finish(db.BuildStatusErrored); err != nil {
			logger.Error("failed-to-mark-build-as-errored", err)
			return startResults{}, fmt.Errorf("finish build: %w", err)
		}

		return startResults{}, nil
	}

	started, err := nextPendingBuild.Start(plan)
	if err != nil {
		logger.Error("failed-to-mark-build-as-started", err)
		return startResults{}, fmt.Errorf("start build: %w", err)
	}

	if !started {
		if err = nextPendingBuild.Finish(db.BuildStatusAborted); err != nil {
			logger.Error("failed-to-mark-build-as-finished", err)
			return startResults{}, fmt.Errorf("finish build: %w", err)
		}

		return startResults{}, nil
	}

	return startResults{
		started:    true,
		needsRetry: false,
	}, nil
}
