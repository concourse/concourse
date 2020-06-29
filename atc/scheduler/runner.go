package scheduler

import (
	"context"
	"fmt"
	"os"
	"runtime/debug"
	"strconv"
	"sync"
	"time"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/metric"
	"github.com/concourse/concourse/tracing"
	"go.opentelemetry.io/otel/api/key"
)

//go:generate counterfeiter . BuildScheduler

type BuildScheduler interface {
	Schedule(
		ctx context.Context,
		logger lager.Logger,
		job db.SchedulerJob,
	) (bool, error)
}

type Runner struct {
	logger     lager.Logger
	jobFactory db.JobFactory
	scheduler  BuildScheduler

	guardJobScheduling chan struct{}
	running            *sync.Map
}

func NewRunner(logger lager.Logger, jobFactory db.JobFactory, scheduler BuildScheduler, maxJobs uint64) *Runner {
	return &Runner{
		logger:     logger,
		jobFactory: jobFactory,
		scheduler:  scheduler,

		guardJobScheduling: make(chan struct{}, maxJobs),
		running:            &sync.Map{},
	}
}

func (s *Runner) Run(ctx context.Context) error {
	sLog := s.logger.Session("run")

	sLog.Debug("start")
	defer sLog.Debug("done")
	spanCtx, span := tracing.StartSpan(ctx, "scheduler.Run", nil)
	defer span.End()

	jobs, err := s.jobFactory.JobsToSchedule()
	if err != nil {
		return fmt.Errorf("find jobs to schedule: %w", err)
	}

	for _, j := range jobs {
		if _, exists := s.running.LoadOrStore(j.ID(), true); exists {
			// already scheduling this job
			continue
		}

		s.guardJobScheduling <- struct{}{}

		jLog := sLog.Session("job", lager.Data{"job": j.Name()})

		go func(job db.SchedulerJob) {
			loggerData := lager.Data{
				"job_id":        strconv.Itoa(job.ID()),
				"job_name":      job.Name(),
				"pipeline_name": job.PipelineName(),
				"team_name":     job.TeamName(),
			}
			defer func() {
				if r := recover(); r != nil {
					err = fmt.Errorf("panic in scheduler run %s: %v", loggerData, r)

					fmt.Fprintf(os.Stderr, "%s\n %s\n", err.Error(), string(debug.Stack()))
					jLog.Error("panic-in-scheduler-run", err)
				}
			}()
			defer func() {
				<-s.guardJobScheduling
				s.running.Delete(job.ID())
			}()

			schedulingLock, acquired, err := job.AcquireSchedulingLock(sLog)
			if err != nil {
				jLog.Error("failed-to-acquire-lock", err)
				return
			}

			if !acquired {
				return
			}

			defer schedulingLock.Release()

			err = s.scheduleJob(spanCtx, sLog, job)
			if err != nil {
				jLog.Error("failed-to-schedule-job", err)
			}
		}(j)
	}

	return nil
}

func (s *Runner) scheduleJob(ctx context.Context, logger lager.Logger, job db.SchedulerJob) error {
	metric.JobsScheduling.Inc()
	defer metric.JobsScheduling.Dec()
	defer metric.JobsScheduled.Inc()

	logger = logger.Session("schedule-job", lager.Data{"job": job.Name()})
	spanCtx, span := tracing.StartSpan(ctx, "schedule-job", tracing.Attrs{
		"team":     job.TeamName(),
		"pipeline": job.PipelineName(),
		"job":      job.Name(),
	})
	defer span.End()

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
		spanCtx,
		logger,
		job,
	)
	if err != nil {
		return fmt.Errorf("schedule job: %w", err)
	}

	span.SetAttributes(key.New("needs-retry").Bool(needsRetry))
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
