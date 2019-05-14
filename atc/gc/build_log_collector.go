package gc

import (
	"context"

	"code.cloudfoundry.org/lager/lagerctx"

	"github.com/concourse/concourse/atc/db"
	"time"
)

type buildLogCollector struct {
	pipelineFactory             db.PipelineFactory
	batchSize                   int
	drainerConfigured           bool
	buildLogRetentionCalculator BuildLogRetentionCalculator
}

func NewBuildLogCollector(
	pipelineFactory db.PipelineFactory,
	batchSize int,
	buildLogRetentionCalculator BuildLogRetentionCalculator,
	drainerConfigured bool,
) Collector {
	return &buildLogCollector{
		pipelineFactory:             pipelineFactory,
		batchSize:                   batchSize,
		drainerConfigured:           drainerConfigured,
		buildLogRetentionCalculator: buildLogRetentionCalculator,
	}
}

func (br *buildLogCollector) Run(ctx context.Context) error {
	logger := lagerctx.FromContext(ctx).Session("build-reaper")

	logger.Debug("start")
	defer logger.Debug("done")

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
			logRetention := br.buildLogRetentionCalculator.BuildLogsToRetain(job)
			if logRetention.Builds == 0 && logRetention.Days == 0 {
				continue
			}

			buildsToConsiderDeleting := []db.Build{}
			until := job.FirstLoggedBuildID() - 1
			limit := br.batchSize

			if job.FirstLoggedBuildID() <= 1 {
				until = 1

				buildsToConsiderDeleting, _, err = job.Builds(
					db.Page{Since: 2, Limit: 1},
				)
				if err != nil {
					logger.Error("failed-to-get-job-build-1-to-delete", err)
					return err
				}

				limit -= len(buildsToConsiderDeleting)
			}

			if limit > 0 {
				moreBuildsToConsiderDeleting, _, err := job.Builds(
					db.Page{Until: until, Limit: limit},
				)
				if err != nil {
					logger.Error("failed-to-get-job-builds-to-delete", err)
					return err
				}

				buildsToConsiderDeleting = append(
					moreBuildsToConsiderDeleting,
					buildsToConsiderDeleting...,
				)
			}

			buildIDsToConsiderDeleting := []int{}
			for _, build := range buildsToConsiderDeleting {
				buildIDsToConsiderDeleting = append(buildIDsToConsiderDeleting, build.ID())
			}

			var firstBuildToRetain int

			if logRetention.Builds > 0 {
				buildsToRetain, _, err := job.Builds(
					db.Page{Limit: logRetention.Builds},
				)

				if err != nil {
					logger.Error("failed-to-get-job-builds-to-retain", err)
					return err
				}

				if len(buildsToRetain) == 0 {
					continue
				}

				firstBuildToRetain = buildsToRetain[len(buildsToRetain)-1].ID()
			}

			buildIDsToDelete := []int{}
			for i := len(buildsToConsiderDeleting) - 1; i >= 0; i-- {
				build := buildsToConsiderDeleting[i]

				if logRetention.Builds > 0 && build.ID() >= firstBuildToRetain || build.IsRunning() {
					break
				}

				if br.drainerConfigured {
					if !build.IsDrained() {
						continue
					}
				}

				if logRetention.Days > 0 {
					if build.EndTime().AddDate(0, 0, logRetention.Days).After(time.Now()) {
						continue
					}
				}

				buildIDsToDelete = append(buildIDsToDelete, build.ID())
			}

			if len(buildIDsToDelete) == 0 {
				continue
			}

			err = pipeline.DeleteBuildEventsByBuildIDs(buildIDsToDelete)
			if err != nil {
				logger.Error("failed-to-delete-build-events", err)
				return err
			}

			err = job.UpdateFirstLoggedBuildID(buildIDsToDelete[len(buildIDsToDelete)-1] + 1)
			if err != nil {
				logger.Error("failed-to-update-first-logged-build-id", err)
				return err
			}
		}
	}

	return nil
}
