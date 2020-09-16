package gc

import (
	"context"

	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagerctx"

	"time"

	"github.com/concourse/concourse/atc/db"
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

func (br *buildLogCollector) Run(ctx context.Context) error {
	logger := lagerctx.FromContext(ctx).Session("build-reaper")

	logger.Debug("start")
	defer logger.Debug("done")

	err := br.pipelineLifecycle.RemoveBuildEventsForDeletedPipelines()
	if err != nil {
		logger.Error("failed-to-remove-build-events-for-deleted-pipelines", err)
		return err
	}

	pipelines, err := br.pipelineFactory.AllPipelines()
	if err != nil {
		logger.Error("failed-to-get-pipelines", err)
		return err
	}

	for _, pipeline := range pipelines {
		if pipeline.Paused() {
			continue
		}

		jobs, err := pipeline.Jobs()
		if err != nil {
			logger.Error("failed-to-get-dashboard", err)
			return err
		}

		for _, job := range jobs {
			err = br.reapLogsOfJob(pipeline, job, logger)
			if err != nil {
				return err
			}
		}
	}

	return nil
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
		"buildsToConsiderDeleting": len(buildsToConsiderDeleting),
	})

	if len(buildsToConsiderDeleting) == 0 {
		return nil
	}

	buildIDsToDelete := []int{}
	toRetainNonSucceededBuildIDs := []int{}
	retainedBuilds := 0
	retainedSucceededBuilds := 0
	firstLoggedBuildID := 0
	for _, build := range buildsToConsiderDeleting {
		// Running build should not be reaped.
		if build.IsRunning() {
			firstLoggedBuildID = build.ID()
			continue
		}

		if logRetention.Days > 0 {
			if !build.EndTime().IsZero() && build.EndTime().AddDate(0, 0, logRetention.Days).Before(time.Now()) {
				logger.Debug("should-reap-due-to-days", lager.Data{"build_id": build.ID()})
				buildIDsToDelete = append(buildIDsToDelete, build.ID())
				continue
			}
		}

		// Before a build is drained, it should not be reaped.
		if br.drainerConfigured {
			if !build.IsDrained() {
				firstLoggedBuildID = build.ID()
				continue
			}
		}

		// If Builds is 0, then all builds are retained, so we don't need to
		// check MinSuccessBuilds at all.
		if logRetention.Builds > 0 {
			if logRetention.MinimumSucceededBuilds > 0 && build.Status() == db.BuildStatusSucceeded {
				if retainedSucceededBuilds < logRetention.MinimumSucceededBuilds {
					retainedBuilds++
					retainedSucceededBuilds++
					firstLoggedBuildID = build.ID()
					continue
				}
			}

			if retainedBuilds < logRetention.Builds {
				retainedBuilds++
				toRetainNonSucceededBuildIDs = append(toRetainNonSucceededBuildIDs, build.ID())
				firstLoggedBuildID = build.ID()
				continue
			}

			buildIDsToDelete = append(buildIDsToDelete, build.ID())
		}
	}

	logger.Debug("after-second-round-filter", lager.Data{
		"retainedBuilds":          retainedBuilds,
		"retainedSucceededBuilds": retainedSucceededBuilds,
	})

	if len(buildIDsToDelete) == 0 {
		logger.Debug("no-builds-to-reap")
		return nil
	}

	// If this happens, firstLoggedBuildID must points to a success build, thus
	// no need to update firstLoggedBuildID.
	if retainedBuilds > logRetention.Builds {
		logger.Debug("more-builds-to-retain", lager.Data{
			"retainedBuilds": retainedBuilds,
		})
		delta := retainedBuilds - logRetention.Builds
		n := len(toRetainNonSucceededBuildIDs)
		for i := 1; i <= delta; i++ {
			buildIDsToDelete = append(buildIDsToDelete, toRetainNonSucceededBuildIDs[n-i])
		}
	}

	logger.Debug("reaping-builds", lager.Data{
		"build-ids": buildIDsToDelete,
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
