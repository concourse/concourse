package scheduler

import (
	"context"
	"errors"
	"time"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/metric"
)

//go:generate counterfeiter . BuildScheduler

type BuildScheduler interface {
	Schedule(
		logger lager.Logger,
		versions *db.VersionsDB,
		job db.Job,
		resources db.Resources,
	) error
}

var errPipelineRemoved = errors.New("pipeline removed")

const maxJobsInFlight = 32

var guardJobScheduling = make(chan struct{}, maxJobsInFlight)

// type Runner struct {
// 	Logger    lager.Logger
// 	Pipeline  db.Pipeline
// 	Scheduler BuildScheduler
// 	Noop      bool
// 	Interval  time.Duration
// }

// func (runner *Runner) Run(signals <-chan os.Signal, ready chan<- struct{}) error {
// 	close(ready)

// 	if runner.Interval == 0 {
// 		panic("unconfigured scheduler interval")
// 	}

// 	runner.Logger.Info("start", lager.Data{
// 		"interval": runner.Interval.String(),
// 	})

// 	defer runner.Logger.Info("done")

// dance:
// 	for {
// 		err := runner.tick(runner.Logger.Session("tick"))
// 		if err != nil {
// 			return err
// 		}

// 		select {
// 		case <-time.After(runner.Interval):
// 		case <-signals:
// 			break dance
// 		}
// 	}

// 	return nil
// }

// func (runner *Runner) tick(logger lager.Logger) error {
// 	if runner.Noop {
// 		return nil
// 	}

// 	start := time.Now()

// 	defer func() {
// 		metric.SchedulingFullDuration{
// 			PipelineName: runner.Pipeline.Name(),
// 			Duration:     time.Since(start),
// 		}.Emit(logger)
// 	}()

// 	found, err := runner.Pipeline.Reload()
// 	if err != nil {
// 		logger.Error("failed-to-update-pipeline-config", err)
// 		return nil
// 	}

// 	if !found {
// 		return errPipelineRemoved
// 	}

// 	versions, err := runner.Pipeline.LoadVersionsDB()
// 	if err != nil {
// 		logger.Error("failed-to-load-versions-db", err)
// 		return err
// 	}

// 	jobs, err := runner.Pipeline.Jobs()
// 	if err != nil {
// 		logger.Error("failed-to-get-jobs", err)
// 		return err
// 	}

// 	for _, job := range jobs {
// 		schedulingLock, acquired, err := job.AcquireSchedulingLock(logger, runner.Interval)
// 		if err != nil {
// 			logger.Error("failed-to-acquire-scheduling-lock", err)
// 			return nil
// 		}

// 		if !acquired {
// 			continue
// 		}

// 		guardJobScheduling <- struct{}{} // would block if guard channel is already filled
// 		go func(j db.Job) {
// 			runner.scheduleJob(logger, schedulingLock, versions, j)
// 			<-guardJobScheduling
// 		}(job)
// 	}

// 	return err
// }

type schedulerRunner struct {
	logger     lager.Logger
	jobFactory db.JobFactory
	interval   time.Duration
}

func (s *schedulerRunner) Run(ctx context.Context) error {
	s.logger.Info("start")
	defer s.logger.Info("end")

	lock, acquired, err := s.jobFactory.AcquireSchedulingLock(s.logger)
	if err != nil {
		s.logger.Error("failed-to-get-scheduling-lock", err)
		return err
	}

	if !acquired {
		s.logger.Debug("scheduling-already-in-progress")
		return nil
	}

	defer lock.Release()

	jobs, err := s.jobFactory.JobsForPipelines()
	if err != nil {
		s.logger.Error("failed-to-get-jobs", err)
		return err
	}

	resources, err := s.resourceFactory.ResourcesForPipelines()
	if err != nil {
		s.logger.Error("failed-to-get-resources-for-each-pipeline", err)
		return err
	}

	for _, jobsWithinPipeline := range jobs {
		for _, job := range jobsWithinPipeline {
			schedulingLock, acquired, err := job.AcquireSchedulingLock(logger, s.Interval)
			if err != nil {
				logger.Error("failed-to-acquire-scheduling-lock", err)
				return nil
			}

			if !acquired {
				continue
			}

			guardJobScheduling <- struct{}{}
			go func(job db.Job) {
				s.scheduleJob(s.logger, job, jobs, resources[job.PipelineID])
				<-guardJobScheduling
			}(job)
		}
	}
	return nil
}

func (s *schedulerRunner) scheduleJob(logger lager.Logger, job db.Job) {

	found, err := job.Reload()
	if err != nil {
		logger.Error("failed-to-update-job-config", err)
		return
	}

	if !found {
		logger.Error("job-not-found", err)
		return
	}

	resources, err := runner.Pipeline.Resources()
	if err != nil {
		logger.Error("failed-to-get-resources", err)
		return
	}

	sLog := logger.Session("scheduling")
	jStart := time.Now()

	err = runner.Scheduler.Schedule(
		sLog,
		versions,
		job,
		resources,
	)

	metric.SchedulingJobDuration{
		PipelineName: runner.Pipeline.Name(),
		JobName:      job.Name(),
		JobID:        job.ID(),
		Duration:     time.Since(jStart),
	}.Emit(sLog)
}
