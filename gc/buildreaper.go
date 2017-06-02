package gc

import (
	"code.cloudfoundry.org/lager"
	"github.com/concourse/atc/db"
)

type BuildReaper interface {
	Run() error
}

type buildReaper struct {
	logger          lager.Logger
	pipelineFactory db.PipelineFactory
	batchSize       int
}

func NewBuildReaper(
	logger lager.Logger,
	pipelineFactory db.PipelineFactory,
	batchSize int,
) BuildReaper {
	return &buildReaper{
		logger:          logger,
		pipelineFactory: pipelineFactory,
		batchSize:       batchSize,
	}
}

func (br *buildReaper) Run() error {
	pipelines, err := br.pipelineFactory.AllPipelines()
	if err != nil {
		br.logger.Error("could-not-get-pipelines", err)
		return err
	}

	for _, pipeline := range pipelines {
		if pipeline.Paused() {
			continue
		}

		jobs, err := pipeline.Jobs()
		if err != nil {
			br.logger.Error("could-not-get-dashboard", err)
			return err
		}

		for _, job := range jobs {
			if job.Config().BuildLogsToRetain == 0 {
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
					br.logger.Error("could-not-get-job-build-1-to-delete", err)
					return err
				}

				limit -= len(buildsToConsiderDeleting)
			}

			if limit > 0 {
				moreBuildsToConsiderDeleting, _, err := job.Builds(
					db.Page{Until: until, Limit: limit},
				)
				if err != nil {
					br.logger.Error("could-not-get-job-builds-to-delete", err)
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

			buildsToRetain, _, err := job.Builds(
				db.Page{Limit: job.Config().BuildLogsToRetain},
			)
			if err != nil {
				br.logger.Error("could-not-get-job-builds-to-retain", err)
				return err
			}

			buildIDsToRetain := []int{}
			for _, build := range buildsToRetain {
				buildIDsToRetain = append(buildIDsToRetain, build.ID())
			}

			if len(buildsToRetain) == 0 {
				continue
			}

			firstBuildToRetain := buildsToRetain[len(buildsToRetain)-1].ID()

			buildIDsToDelete := []int{}
			for i := len(buildsToConsiderDeleting) - 1; i >= 0; i-- {
				build := buildsToConsiderDeleting[i]

				if build.ID() >= firstBuildToRetain || build.IsRunning() {
					break
				}

				buildIDsToDelete = append(buildIDsToDelete, build.ID())
			}

			if len(buildIDsToDelete) == 0 {
				continue
			}

			err = pipeline.DeleteBuildEventsByBuildIDs(buildIDsToDelete)
			if err != nil {
				br.logger.Error("could-not-delete-build-events", err)
				return err
			}

			err = job.UpdateFirstLoggedBuildID(buildIDsToDelete[len(buildIDsToDelete)-1] + 1)
			if err != nil {
				br.logger.Error("could-not-update-first-logged-build-id", err)
				return err
			}
		}
	}

	return nil
}
