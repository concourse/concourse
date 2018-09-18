package gc

import (
	"sync"
	"time"

	"code.cloudfoundry.org/lager"

	"github.com/concourse/atc/worker"
)

type workerJobRunner struct {
	workerProvider worker.WorkerProvider

	workers             map[string]worker.Worker
	workersL            *sync.Mutex
	workersSyncInterval time.Duration
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
	workerProvider worker.WorkerProvider,
	workersSyncInterval time.Duration,
) WorkerJobRunner {
	runner := &workerJobRunner{
		workerProvider: workerProvider,

		workers:             map[string]worker.Worker{},
		workersL:            &sync.Mutex{},
		workersSyncInterval: workersSyncInterval,
	}

	go runner.syncWorkersLoop(logger)

	return runner
}

func (runner *workerJobRunner) Try(logger lager.Logger, workerName string, job Job) {
	logger = logger.Session("queue", lager.Data{
		"worker-name": workerName,
		"job-name":    job.Name(),
	})

	runner.workersL.Lock()
	workerClient, found := runner.workers[workerName]
	runner.workersL.Unlock()

	if !found {
		// drop the job on the floor; it'll be queued up again later
		//TODO: move this over to container collector, as we only need the
		//worker to be running if the container to be GC'd is hijacked
		//so we shouldn't be dropping every job for non-running workers
		logger.Info("worker-not-found")
		return
	}

	go func() {
		job.Run(workerClient)
	}()
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
	workers, err := runner.workerProvider.RunningWorkers(logger)
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
