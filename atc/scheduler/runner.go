package scheduler

import (
	"context"
	"fmt"
	"time"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/db/lock"
	"github.com/concourse/concourse/atc/metric"
	"github.com/concourse/concourse/atc/scheduler/algorithm"
	"github.com/hashicorp/go-multierror"
)

//go:generate counterfeiter . BuildScheduler

type BuildScheduler interface {
	Schedule(
		ctx context.Context,
		logger lager.Logger,
		pipeline db.Pipeline,
		job db.Job,
		resources db.Resources,
		relatedJobIDs algorithm.NameToIDMap,
	) (bool, error)
}

type schedulerRunner struct {
	logger             lager.Logger
	jobFactory         db.JobFactory
	scheduler          BuildScheduler
	guardJobScheduling chan struct{}
}

func NewRunner(logger lager.Logger, jobFactory db.JobFactory, scheduler BuildScheduler, maxJobs uint64) Runner {
	newGuardJobScheduling := make(chan struct{}, maxJobs)
	return &schedulerRunner{
		logger:             logger,
		jobFactory:         jobFactory,
		scheduler:          scheduler,
		guardJobScheduling: newGuardJobScheduling,
	}
}

func (s *schedulerRunner) Run(ctx context.Context) error {
	s.logger.Info("start")
	defer s.logger.Info("end")

	jobs, err := s.jobFactory.JobsToSchedule()
	if err != nil {
		return fmt.Errorf("find jobs to schedule: %w", err)
	}

	pipelineIDToPipeline, pipelineIDToJobs, err := s.constructPipelineIDMaps(jobs)
	if err != nil {
		return err
	}

	errGroup := new(multierror.Group)
	for pipelineID, jobsToSchedule := range pipelineIDToJobs {
		pipeline := pipelineIDToPipeline[pipelineID]

		err := s.schedulePipeline(ctx, errGroup, pipeline, jobsToSchedule)
		if err != nil {
			s.logger.Error("failed-to-schedule-pipeline", err, lager.Data{"pipeline": pipeline.Name()})
		}
	}

	return errGroup.Wait().ErrorOrNil()
}

func (s *schedulerRunner) schedulePipeline(ctx context.Context, errGroup *multierror.Group, pipeline db.Pipeline, jobsToSchedule db.Jobs) error {
	logger := s.logger.Session("pipeline", lager.Data{"pipeline": pipeline.Name()})

	resources, err := pipeline.Resources()
	if err != nil {
		return fmt.Errorf("find resources: %w", err)
	}

	jobs, err := pipeline.Jobs()
	if err != nil {
		return fmt.Errorf("find jobs: %w", err)
	}

	jobsMap := map[string]int{}
	for _, job := range jobs {
		jobsMap[job.Name()] = job.ID()
	}

	for _, job := range jobsToSchedule {
		schedulingLock, acquired, err := job.AcquireSchedulingLock(logger)
		if err != nil {
			return fmt.Errorf("acquire job lock: %w", err)
		}

		if !acquired {
			continue
		}

		// shadow loop var for the goroutine closure
		job := job

		s.guardJobScheduling <- struct{}{}
		errGroup.Go(func() error {
			defer func() {
				<-s.guardJobScheduling
			}()

			return s.scheduleJob(ctx, logger, schedulingLock, pipeline, job, resources, jobsMap)
		})
	}

	return nil
}

func (s *schedulerRunner) scheduleJob(ctx context.Context, logger lager.Logger, schedulingLock lock.Lock, pipeline db.Pipeline, job db.Job, resources db.Resources, jobs algorithm.NameToIDMap) error {
	logger = logger.Session("schedule-job", lager.Data{"job": job.Name()})

	logger.Debug("schedule")

	defer schedulingLock.Release()

	// Grabs out the requested time that triggered off the job schedule in
	// order to set the last scheduled to the exact time of this triggering
	// request
	requestedTime := job.ScheduleRequestedTime()

	found, err := job.Reload()
	if err != nil {
		return fmt.Errorf("reload job: %w", err)
	}

	if !found {
		logger.Debug("could-not-find-job-to-reload")
		return nil
	}

	jStart := time.Now()

	needsRetry, err := s.scheduler.Schedule(
		ctx,
		logger,
		pipeline,
		job,
		resources,
		jobs,
	)
	if err != nil {
		return fmt.Errorf("schedule job: %w", err)
	}

	if !needsRetry {
		err = job.UpdateLastScheduled(requestedTime)
		if err != nil {
			logger.Error("failed-to-update-last-scheduled", err, lager.Data{"job": job.Name()})
			return fmt.Errorf("update last scheduled: %w", err)
		}
	}

	metric.SchedulingJobDuration{
		PipelineName: job.PipelineName(),
		JobName:      job.Name(),
		JobID:        job.ID(),
		Duration:     time.Since(jStart),
	}.Emit(logger)

	return nil
}

func (s *schedulerRunner) constructPipelineIDMaps(jobs db.Jobs) (map[int]db.Pipeline, map[int]db.Jobs, error) {
	pipelineIDToPipeline := make(map[int]db.Pipeline)
	pipelineIDToJobs := make(map[int]db.Jobs)

	for _, job := range jobs {
		pipelineID := job.PipelineID()

		_, found := pipelineIDToPipeline[pipelineID]
		if !found {
			pipeline, found, err := job.Pipeline()
			if err != nil {
				return nil, nil, fmt.Errorf("find pipeline for job: %w", err)
			}

			if !found {
				s.logger.Info("could-not-find-pipeline-for-job", lager.Data{"job": job.Name()})
				continue
			}

			pipelineIDToPipeline[pipelineID] = pipeline
		}

		pipelineIDToJobs[pipelineID] = append(pipelineIDToJobs[pipelineID], job)
	}

	return pipelineIDToPipeline, pipelineIDToJobs, nil
}
