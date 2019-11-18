package scheduler

import (
	"context"
	"errors"
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
	) error
}

var errPipelineRemoved = errors.New("pipeline removed")

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
		s.logger.Error("failed-to-get-pipelines-to-schedule", err)
		return err
	}

	pipelineIDToPipeline := make(map[int]db.Pipeline)
	pipelineIDToJobs := make(map[int]db.Jobs)
	for _, job := range jobs {
		pipelineID := job.PipelineID()

		_, found := pipelineIDToPipeline[pipelineID]
		if !found {
			pipeline, err := job.Pipeline()
			if err != nil {
				panic("TODO")
			}

			pipelineIDToPipeline[pipelineID] = pipeline
		}

		pipelineIDToJobs[pipelineID] = append(pipelineIDToJobs[pipelineID], job)
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
		logger.Error("failed-to-get-resources", err)
		return err
	}

	jobs, err := pipeline.Jobs()
	if err != nil {
		logger.Error("failed-to-get-jobs", err)
		return err
	}

	jobsMap := map[string]int{}

	for _, job := range jobs {
		jobsMap[job.Name()] = job.ID()
	}

	for _, job := range jobsToSchedule {
		schedulingLock, acquired, err := job.AcquireSchedulingLock(logger)
		if err != nil {
			logger.Error("failed-to-acquire-scheduling-lock", err)
			return nil
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

			err := s.scheduleJob(logger, schedulingLock, pipeline, job, resources, jobsMap)
			if err != nil {
				logger.Error("failed-to-request-schedule-on-job", err, lager.Data{"pipeline": job.PipelineName(), "job": job.Name()})
			} else {
				// If scheduling the job fails, the last scheduled value will not be
				// updated which will result in the job being scheduled again on the next
				// interval. This allows jobs that failed to schedule to be attempted again.
				err = job.UpdateLastScheduled(requestedTime)
				if err != nil {
					s.logger.Error("failed-to-update-last-scheduled", err, lager.Data{"pipeline": pipeline.Name()})
				}
			}

			<-s.guardJobScheduling
		}(job)
	}

	return nil
}

func (s *schedulerRunner) scheduleJob(logger lager.Logger, schedulingLock lock.Lock, pipeline db.Pipeline, currentJob db.Job, resources db.Resources, jobs algorithm.NameToIDMap) error {
	logger = logger.Session("job", lager.Data{"job": currentJob.Name()})

	defer schedulingLock.Release()

	found, err := currentJob.Reload()
	if err != nil {
		logger.Error("failed-to-update-job-config", err)
		return err
	}

	if !found {
		logger.Error("job-not-found", err)
		return err
	}

	sLog := logger.Session("scheduling")
	jStart := time.Now()

	err = s.scheduler.Schedule(
		sLog,
		pipeline,
		currentJob,
		resources,
		jobs,
	)
	if err != nil {
		logger.Error("failed-to-schedule", err)
		return err
	}

	metric.SchedulingJobDuration{
		PipelineName: currentJob.PipelineName(),
		JobName:      currentJob.Name(),
		JobID:        currentJob.ID(),
		Duration:     time.Since(jStart),
	}.Emit(sLog)

	return nil
}
