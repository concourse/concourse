package gc

import (
	"sync"
	"time"

	"code.cloudfoundry.org/lager"

	"github.com/concourse/atc/worker"
)

type workerJobRunner struct {
	workerPool worker.Client

	maxJobsPerWorker int

	workers             map[string]worker.Worker
	workersL            *sync.Mutex
	workersSyncInterval time.Duration

	workerJobs map[string]int

	jobsL          *sync.Mutex
	inFlightJobs   map[string]struct{}
	dropMetricFunc func(lager.Logger, string)
}

type Job interface {
	Name() string
	Run(worker.Worker)
}

type JobFunc func(worker.Worker)

func (f JobFunc) Name() string { return "" }

func (f JobFunc) Run(workerClient worker.Worker) {
	f(workerClient)
}

//go:generate counterfeiter . WorkerJobRunner

type WorkerJobRunner interface {
	Try(lager.Logger, string, Job)
}

func NewWorkerJobRunner(
	logger lager.Logger,
	workerPool worker.Client,
	workersSyncInterval time.Duration,
	maxJobsPerWorker int,
	dropMetricFunc func(lager.Logger, string),
) WorkerJobRunner {
	runner := &workerJobRunner{
		workerPool: workerPool,

		maxJobsPerWorker: maxJobsPerWorker,

		workers:             map[string]worker.Worker{},
		workersL:            &sync.Mutex{},
		workersSyncInterval: workersSyncInterval,

		workerJobs:     map[string]int{},
		jobsL:          &sync.Mutex{},
		inFlightJobs:   map[string]struct{}{},
		dropMetricFunc: dropMetricFunc,
	}

	go runner.syncWorkersLoop(logger)

	return runner
}

func (runner *workerJobRunner) Try(logger lager.Logger, workerName string, job Job) {
	logger = logger.Session("queue", lager.Data{
		"worker-name": workerName,
	})

	runner.workersL.Lock()
	workerClient, found := runner.workers[workerName]
	runner.workersL.Unlock()

	if !found {
		// drop the job on the floor; it'll be queued up again later
		logger.Info("worker-not-found")
		return
	}

	if !runner.startJob(job.Name(), workerName) {
		logger.Debug("job-limit-reached")
		runner.dropMetricFunc(logger, workerName)

		return
	}

	go func() {
		defer runner.finishJob(job.Name(), workerName)
		job.Run(workerClient)
	}()
}

func (runner *workerJobRunner) startJob(jobName, workerName string) bool {
	runner.jobsL.Lock()
	defer runner.jobsL.Unlock()

	if runner.workerJobs[workerName] == runner.maxJobsPerWorker {
		return false
	}

	if jobName != "" {
		_, inFlight := runner.inFlightJobs[jobName]
		if inFlight {
			return false
		}

		runner.inFlightJobs[jobName] = struct{}{}
	}

	runner.workerJobs[workerName]++

	return true
}

func (runner *workerJobRunner) finishJob(jobName, workerName string) {
	runner.jobsL.Lock()
	if runner.workerJobs[workerName] == 1 {
		delete(runner.workerJobs, workerName)
	} else {
		runner.workerJobs[workerName]--
	}

	if jobName != "" {
		delete(runner.inFlightJobs, jobName)
	}

	runner.jobsL.Unlock()
}

func (runner *workerJobRunner) syncWorkersLoop(logger lager.Logger) {
	runner.syncWorkers(logger)

	ticker := time.NewTicker(runner.workersSyncInterval)

	for {
		select {
		case <-ticker.C:
			runner.syncWorkers(logger)
		}
	}
}

func (runner *workerJobRunner) syncWorkers(logger lager.Logger) {
	workers, err := runner.workerPool.RunningWorkers(logger)
	if err != nil {
		logger.Error("failed-to-get-running-workers", err)
		return
	}

	workerMap := map[string]worker.Worker{}
	for _, worker := range workers {
		workerMap[worker.Name()] = worker
	}

	runner.workersL.Lock()
	runner.workers = workerMap
	runner.workersL.Unlock()
}
