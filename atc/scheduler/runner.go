package scheduler

import (
	"context"
	"fmt"
	"sync"
	"time"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/metric"
	"github.com/concourse/concourse/atc/scheduler/algorithm"
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
	logger     lager.Logger
	jobFactory db.JobFactory
	scheduler  BuildScheduler

	guardJobScheduling chan struct{}
	running            *sync.Map
}

func NewRunner(logger lager.Logger, jobFactory db.JobFactory, scheduler BuildScheduler, maxJobs uint64) Runner {
	newGuardJobScheduling := make(chan struct{}, maxJobs)
	return &schedulerRunner{
		logger:     logger,
		jobFactory: jobFactory,
		scheduler:  scheduler,

		guardJobScheduling: newGuardJobScheduling,
		running:            &sync.Map{},
	}
}

func (s *schedulerRunner) Run(ctx context.Context) error {
	sLog := s.logger.Session("run")

	sLog.Debug("start")
	defer sLog.Debug("done")

	jobs, err := s.jobFactory.JobsToSchedule()
	if err != nil {
		return fmt.Errorf("find jobs to schedule: %w", err)
	}

	pipelineIDToPipeline, pipelineIDToJobs, err := s.constructPipelineIDMaps(jobs)
	if err != nil {
		return err
	}

	for pipelineID, jobsToSchedule := range pipelineIDToJobs {
		pipeline := pipelineIDToPipeline[pipelineID]

		pLog := s.logger.Session("pipeline", lager.Data{"pipeline": pipeline.Name()})

		err := s.schedulePipeline(ctx, pLog, pipeline, jobsToSchedule)
		if err != nil {
			pLog.Error("failed-to-schedule", err)
		}
	}

	return nil
}

func (s *schedulerRunner) schedulePipeline(ctx context.Context, logger lager.Logger, pipeline db.Pipeline, jobsToSchedule db.Jobs) error {
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

	for _, j := range jobsToSchedule {
		if _, exists := s.running.LoadOrStore(j.ID(), true); exists {
			// already scheduling this job
			continue
		}

		s.guardJobScheduling <- struct{}{}

		jLog := logger.Session("job", lager.Data{"job": j.Name()})

		go func(job db.Job) {
			defer func() {
				<-s.guardJobScheduling
				s.running.Delete(job.ID())
			}()

			schedulingLock, acquired, err := job.AcquireSchedulingLock(logger)
			if err != nil {
				jLog.Error("failed-to-acquire-lock", err)
				return
			}

			if !acquired {
				return
			}

			defer schedulingLock.Release()

			err = s.scheduleJob(ctx, logger, pipeline, job, resources, jobsMap)
			if err != nil {
				jLog.Error("failed-to-schedule-job", err)
			}
		}(j)
	}

	return nil
}

func (s *schedulerRunner) scheduleJob(ctx context.Context, logger lager.Logger, pipeline db.Pipeline, job db.Job, resources db.Resources, jobs algorithm.NameToIDMap) error {
	logger = logger.Session("schedule-job", lager.Data{"job": job.Name()})

	logger.Debug("schedule")

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
