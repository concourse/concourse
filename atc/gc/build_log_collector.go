package gc

import (
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagerctx"
	"context"
	"github.com/concourse/concourse/atc/component"
	"github.com/concourse/concourse/atc/db"
	"time"
)

type buildLogCollector struct {
	pipelineFactory             db.PipelineFactory
	pipelineLifecycle           db.PipelineLifecycle
	batchSize                   int
	drainerConfigured           bool
	buildLogRetentionCalculator BuildLogRetentionCalculator
}

func NewBuildLogCollector(
	pipelineFactory db.PipelineFactory,
	pipelineLifecycle db.PipelineLifecycle,
	batchSize int,
	buildLogRetentionCalculator BuildLogRetentionCalculator,
	drainerConfigured bool,
) *buildLogCollector {
	return &buildLogCollector{
		pipelineFactory:             pipelineFactory,
		pipelineLifecycle:           pipelineLifecycle,
		batchSize:                   batchSize,
		drainerConfigured:           drainerConfigured,
		buildLogRetentionCalculator: buildLogRetentionCalculator,
	}
}

func (br *buildLogCollector) Run(ctx context.Context, _ string) (component.RunResult, error) {
	logger := lagerctx.FromContext(ctx).Session("build-reaper")

	logger.Debug("start")
	defer logger.Debug("done")

	err := br.pipelineLifecycle.RemoveBuildEventsForDeletedPipelines()
	if err != nil {
		logger.Error("failed-to-remove-build-events-for-deleted-pipelines", err)
		return nil, err
	}

	pipelines, err := br.pipelineFactory.AllPipelines()
	if err != nil {
		logger.Error("failed-to-get-pipelines", err)
		return nil, err
	}

	for _, pipeline := range pipelines {
		if pipeline.Paused() {
			continue
		}

		jobs, err := pipeline.Jobs()
		if err != nil {
			logger.Error("failed-to-get-dashboard", err)
			return nil, err
		}

		for _, job := range jobs {
			if job.Paused() {
				continue
			}

			err = br.reapLogsOfJob(pipeline, job, logger)
			if err != nil {
				return nil, err
			}
		}
	}

	return nil, nil
}

func (br *buildLogCollector) reapLogsOfJob(pipeline db.Pipeline,
	job db.Job,
	logger lager.Logger) error {

	jobConfig, err := job.Config()
	if err != nil {
		logger.Error("failed-to-get-job-config", err)
		return err
	}

	logRetention := br.buildLogRetentionCalculator.BuildLogsToRetain(jobConfig)
	if logRetention.Builds == 0 && logRetention.Days == 0 {
		return nil
	}

	buildsToConsiderDeleting := []db.Build{}

	from := job.FirstLoggedBuildID()
	limit := br.batchSize
	page := &db.Page{From: &from, Limit: limit}
	for page != nil {
		builds, pagination, err := job.Builds(*page)
		if err != nil {
			logger.Error("failed-to-get-job-builds-to-delete", err)
			return err
		}

		buildsOfBatch := []db.Build{}
		for _, build := range builds {
			// Ignore reaped builds
			if !build.ReapTime().IsZero() {
				continue
			}

			buildsOfBatch = append(buildsOfBatch, build)
		}
		buildsToConsiderDeleting = append(buildsOfBatch, buildsToConsiderDeleting...)

		page = pagination.Newer
	}

	logger.Debug("after-first-round-filter", lager.Data{
		"builds_to_consider_deleting": len(buildsToConsiderDeleting),
	})

	if len(buildsToConsiderDeleting) == 0 {
		return nil
	}

	buildIDsToDelete := []int{}
	candidateBuildIDsToKeep := []int{}
	retainedBuilds := 0
	retainedSucceededBuilds := 0
	firstLoggedBuildID := 0
	for _, build := range buildsToConsiderDeleting {
		// Running build should not be reaped.
		if build.IsRunning() {
			firstLoggedBuildID = build.ID()
			continue
		}

		// Before a build is drained, it should not be reaped.
		if br.drainerConfigured {
			if !build.IsDrained() {
				firstLoggedBuildID = build.ID()
				continue
			}
		}

		maxBuildsRetained := retainedBuilds >= logRetention.Builds
		buildHasExpired := !build.EndTime().IsZero() && build.EndTime().AddDate(0, 0, logRetention.Days).Before(time.Now())


		if logRetention.Builds != 0 {
			if logRetention.MinimumSucceededBuilds != 0 {
				if build.Status() == db.BuildStatusSucceeded && retainedSucceededBuilds < logRetention.MinimumSucceededBuilds {
					retainedBuilds++
					retainedSucceededBuilds++
					firstLoggedBuildID = build.ID()
					continue
				}
			}

			if !maxBuildsRetained {
				retainedBuilds++
				candidateBuildIDsToKeep = append(candidateBuildIDsToKeep, build.ID())
				firstLoggedBuildID = build.ID()
				continue
			}
		}

		if logRetention.Days != 0 {
			if !buildHasExpired {
				retainedBuilds++
				candidateBuildIDsToKeep = append(candidateBuildIDsToKeep, build.ID())
				firstLoggedBuildID = build.ID()
				continue
			}
		}

		// at this point, we haven't met all of the enabled conditions, so here we can reap
		buildIDsToDelete = append(buildIDsToDelete, build.ID())

	}

	logger.Debug("after-second-round-filter", lager.Data{
		"retained_builds":           retainedBuilds,
		"retained_succeeded_builds": retainedSucceededBuilds,
	})

	if len(buildIDsToDelete) == 0 {
		logger.Debug("no-builds-to-reap")
		return nil
	}

	// If we exceeded the maximum number of builds we should delete the oldest candidates
	if logRetention.Builds != 0 && retainedBuilds > logRetention.Builds {
		logger.Debug("more-builds-to-retain", lager.Data{
			"retained_builds": retainedBuilds,
		})
		delta := retainedBuilds - logRetention.Builds
		n := len(candidateBuildIDsToKeep)
		for i := 1; i <= delta; i++ {
			buildIDsToDelete = append(buildIDsToDelete, candidateBuildIDsToKeep[n-i])
		}
	}

	logger.Debug("reaping-builds", lager.Data{
		"build_ids": buildIDsToDelete,
	})

	err = pipeline.DeleteBuildEventsByBuildIDs(buildIDsToDelete)
	if err != nil {
		logger.Error("failed-to-delete-build-events", err)
		return err
	}

	if firstLoggedBuildID > job.FirstLoggedBuildID() {
		err = job.UpdateFirstLoggedBuildID(firstLoggedBuildID)
		if err != nil {
			logger.Error("failed-to-update-first-logged-build-id", err)
			return err
		}
	}

	return nil
}
