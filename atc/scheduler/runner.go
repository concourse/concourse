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
)

//go:generate counterfeiter . BuildScheduler

type BuildScheduler interface {
	Schedule(
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

	for pipelineID, jobs := range pipelineIDToJobs {
		pipeline := pipelineIDToPipeline[pipelineID]

		go func(jobsToSchedule db.Jobs) {
			err := s.schedulePipeline(pipeline, jobsToSchedule)
			if err != nil {
				s.logger.Error("failed-to-schedule-pipeline", err, lager.Data{"pipeline": pipeline.Name()})
				return
			}
		}(jobs)
	}

	return nil
}

func (s *schedulerRunner) schedulePipeline(pipeline db.Pipeline, jobsToSchedule db.Jobs) error {
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

		s.guardJobScheduling <- struct{}{}
		go func(job db.Job) {
			// Grabs out the requested time that triggered off the job schedule in
			// order to set the last scheduled to the exact time of this triggering
			// request
			requestedTime := job.ScheduleRequestedTime()

			needsRetry, err := s.scheduleJob(logger, schedulingLock, pipeline, job, resources, jobsMap)
			if err != nil {
				logger.Error("failed-to-schedule-job", err, lager.Data{"job": job.Name()})
			} else {
				if !needsRetry {
					err = job.UpdateLastScheduled(requestedTime)
					if err != nil {
						logger.Error("failed-to-update-last-scheduled", err, lager.Data{"job": job.Name()})
					}
				}
			}

			<-s.guardJobScheduling
		}(job)
	}

	return nil
}

func (s *schedulerRunner) scheduleJob(logger lager.Logger, schedulingLock lock.Lock, pipeline db.Pipeline, currentJob db.Job, resources db.Resources, jobs algorithm.NameToIDMap) (bool, error) {
	logger = logger.Session("schedule-job", lager.Data{"job": currentJob.Name()})

	logger.Debug("schedule")

	defer schedulingLock.Release()

	found, err := currentJob.Reload()
	if err != nil {
		return false, fmt.Errorf("reload job: %w", err)
	}

	if !found {
		logger.Debug("could-not-find-job-to-reload")
		return false, nil
	}

	jStart := time.Now()

	needsRetry, err := s.scheduler.Schedule(
		logger,
		pipeline,
		currentJob,
		resources,
		jobs,
	)
	if err != nil {
		return false, fmt.Errorf("schedule job: %w", err)
	}

	metric.SchedulingJobDuration{
		PipelineName: currentJob.PipelineName(),
		JobName:      currentJob.Name(),
		JobID:        currentJob.ID(),
		Duration:     time.Since(jStart),
	}.Emit(logger)

	return needsRetry, nil
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
