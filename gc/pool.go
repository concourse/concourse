package gc

import (
	"sync"
	"time"

	"code.cloudfoundry.org/lager"

	"github.com/concourse/atc/worker"
)

type WorkerPool struct {
	workerPool       worker.Client
	maxJobsPerWorker int

	workers  map[string]worker.Worker
	workersL *sync.Mutex

	workerJobs  map[string]int
	workerJobsL *sync.Mutex
}

type Job interface {
	Run(worker.Worker)
}

type JobFunc func(worker.Worker)

func (f JobFunc) Run(workerClient worker.Worker) {
	f(workerClient)
}

func NewWorkerPool(logger lager.Logger, workerPool worker.Client, maxJobsPerWorker int) *WorkerPool {
	pool := &WorkerPool{
		workerPool:       workerPool,
		maxJobsPerWorker: maxJobsPerWorker,

		workers:  map[string]worker.Worker{},
		workersL: &sync.Mutex{},

		workerJobs:  map[string]int{},
		workerJobsL: &sync.Mutex{},
	}

	go pool.syncWorkersLoop(logger)

	return pool
}

func (pool *WorkerPool) Queue(logger lager.Logger, workerName string, job Job) {
	logger = logger.Session("queue", lager.Data{
		"worker-name": workerName,
	})

	pool.workersL.Lock()
	workerClient, found := pool.workers[workerName]
	pool.workersL.Unlock()

	if !found {
		// drop the job on the floor; it'll be queued up again later
		logger.Info("worker-not-found")
		return
	}

	if !pool.startJob(workerName) {
		logger.Debug("job-limit-reached")
		// drop the job on the floor; it'll be queued up again later
		return
	}

	go func() {
		defer pool.finishJob(workerName)
		job.Run(workerClient)
	}()
}

func (pool *WorkerPool) startJob(workerName string) bool {
	pool.workerJobsL.Lock()
	defer pool.workerJobsL.Unlock()

	if pool.workerJobs[workerName] == pool.maxJobsPerWorker {
		return false
	}

	pool.workerJobs[workerName]++

	return true
}

func (pool *WorkerPool) finishJob(workerName string) {
	pool.workerJobsL.Lock()
	if pool.workerJobs[workerName] == 1 {
		delete(pool.workerJobs, workerName)
	} else {
		pool.workerJobs[workerName]--
	}
	pool.workerJobsL.Unlock()
}

func (pool *WorkerPool) syncWorkersLoop(logger lager.Logger) {
	pool.syncWorkers(logger)

	ticker := time.NewTicker(30 * time.Second) // XXX: parameterize same as default worker TTL (...which might actually live on the worker side...)

	for {
		select {
		case <-ticker.C:
			pool.syncWorkers(logger)
		}
	}
}

func (pool *WorkerPool) syncWorkers(logger lager.Logger) {
	workers, err := pool.workerPool.RunningWorkers(logger)
	if err != nil {
		logger.Error("failed-to-get-running-workers", err)
		return
	}

	workerMap := map[string]worker.Worker{}
	for _, worker := range workers {
		workerMap[worker.Name()] = worker
	}

	pool.workersL.Lock()
	pool.workers = workerMap
	pool.workersL.Unlock()
}
