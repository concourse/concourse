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
	pipelineFactory    db.PipelineFactory
	scheduler          BuildScheduler
	guardJobScheduling chan struct{}
}

func NewRunner(logger lager.Logger, pipelineFactory db.PipelineFactory, scheduler BuildScheduler, maxJobs uint64) Runner {
	newGuardJobScheduling := make(chan struct{}, maxJobs)
	return &schedulerRunner{
		logger:             logger,
		pipelineFactory:    pipelineFactory,
		scheduler:          scheduler,
		guardJobScheduling: newGuardJobScheduling,
	}
}

func (s *schedulerRunner) Run(ctx context.Context) error {
	s.logger.Info("start")
	defer s.logger.Info("end")

	pipelines, err := s.pipelineFactory.PipelinesToSchedule()
	if err != nil {
		s.logger.Error("failed-to-get-pipelines-to-schedule", err)
		return err
	}

	for _, pipeline := range pipelines {
		// Grabs out the requested time that triggered off the pipeline schedule in
		// order to set the last scheduled to the exact time of this triggering
		// request
		requestedTime := pipeline.RequestedTime()

		go func(pipeline db.Pipeline, requestedTime time.Time) {
			err = s.schedulePipeline(pipeline)
			if err != nil {
				s.logger.Error("failed-to-schedule-pipeline", err, lager.Data{"pipeline": pipeline.Name()})
				return
			}

			err = pipeline.UpdateLastScheduled(requestedTime)
			if err != nil {
				s.logger.Error("failed-to-update-last-scheduled", err, lager.Data{"pipeline": pipeline.Name()})
			}
		}(pipeline, requestedTime)
	}

	return nil
}

func (s *schedulerRunner) schedulePipeline(pipeline db.Pipeline) error {
	logger := s.logger.Session("pipeline", lager.Data{"pipeline": pipeline.Name()})

	jobs, err := pipeline.Jobs()
	if err != nil {
		logger.Error("failed-to-get-jobs", err)
		return err
	}

	resources, err := pipeline.Resources()
	if err != nil {
		logger.Error("failed-to-get-resources", err)
		return err
	}

	jobsMap := map[string]int{}

	for _, job := range jobs {
		jobsMap[job.Name()] = job.ID()
	}

	for _, job := range jobs {
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
			err = s.scheduleJob(logger, schedulingLock, pipeline, job, resources, jobsMap)
			if err != nil {
				err = pipeline.RequestSchedule()
				if err != nil {
					// XXX if requesting schedule fails because of a connection failure,
					// we have no way of retrying the scheduler
					logger.Error("failed-to-request-schedule-on-job", err, lager.Data{"pipeline": job.PipelineName(), "job": job.Name()})
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
