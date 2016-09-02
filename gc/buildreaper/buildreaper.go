package buildreaper

import (
	"code.cloudfoundry.org/lager"
	"github.com/concourse/atc/db"
)

//go:generate counterfeiter . BuildReaperDB

type BuildReaperDB interface {
	GetAllPipelines() ([]db.SavedPipeline, error)
	DeleteBuildEventsByBuildIDs(buildIDs []int) error
}

type BuildReaper interface {
	Run() error
}

type buildReaper struct {
	logger            lager.Logger
	db                BuildReaperDB
	pipelineDBFactory db.PipelineDBFactory
	batchSize         int
}

func NewBuildReaper(
	logger lager.Logger,
	db BuildReaperDB,
	pipelineDBFactory db.PipelineDBFactory,
	batchSize int,
) BuildReaper {
	return &buildReaper{
		logger:            logger,
		db:                db,
		pipelineDBFactory: pipelineDBFactory,
		batchSize:         batchSize,
	}
}

func (br *buildReaper) Run() error {
	pipelines, err := br.db.GetAllPipelines()
	if err != nil {
		br.logger.Error("could-not-get-active-pipelines", err)
		return err
	}

	for _, pipeline := range pipelines {
		if pipeline.Paused {
			continue
		}

		pipelineDB := br.pipelineDBFactory.Build(pipeline)

		jobs, _, err := pipelineDB.GetDashboard()
		if err != nil {
			br.logger.Error("could-not-get-dashboard", err)
			return err
		}

		for _, job := range jobs {
			if job.Job.Config.BuildLogsToRetain == 0 {
				continue
			}

			buildsToConsiderDeleting := []db.Build{}
			until := job.Job.FirstLoggedBuildID - 1
			limit := br.batchSize

			if job.Job.FirstLoggedBuildID <= 1 {
				until = 1

				buildsToConsiderDeleting, _, err = pipelineDB.GetJobBuilds(
					job.Job.Name,
					db.Page{Since: 2, Limit: 1},
				)
				if err != nil {
					br.logger.Error("could-not-get-job-build-1-to-delete", err)
					return err
				}

				limit -= len(buildsToConsiderDeleting)
			}

			if limit > 0 {
				moreBuildsToConsiderDeleting, _, err := pipelineDB.GetJobBuilds(
					job.Job.Name,
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

			buildsToRetain, _, err := pipelineDB.GetJobBuilds(
				job.Job.Name,
				db.Page{Limit: job.Job.Config.BuildLogsToRetain},
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

			err = br.db.DeleteBuildEventsByBuildIDs(buildIDsToDelete)
			if err != nil {
				br.logger.Error("could-not-delete-build-events", err)
				return err
			}

			err = pipelineDB.UpdateFirstLoggedBuildID(job.Job.Name, buildIDsToDelete[len(buildIDsToDelete)-1]+1)
			if err != nil {
				br.logger.Error("could-not-update-first-logged-build-id", err)
				return err
			}
		}
	}

	return nil
}
