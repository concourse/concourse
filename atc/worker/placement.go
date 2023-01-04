package worker

import (
	"errors"
	"fmt"
	"math/rand"
	"sort"
	"strings"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/runtime"
)

type PlacementOptions struct {
	Strategies                   []string `long:"container-placement-strategy" default:"volume-locality" choice:"volume-locality" choice:"random" choice:"fewest-build-containers" choice:"limit-active-tasks" choice:"limit-active-containers" choice:"limit-active-volumes" description:"Method by which a worker is selected during container placement. If multiple methods are specified, they will be applied in order. Random strategy should only be used alone."`
	NoInputStrategies            []string `long:"no-input-container-placement-strategy"  choice:"random" choice:"fewest-build-containers" choice:"limit-active-tasks" choice:"limit-active-containers" choice:"limit-active-volumes" description:"A second container placement strategy that will only be used for get and nested check steps."`
	CheckStrategies              []string `long:"check-container-placement-strategy" default:"random" choice:"random" choice:"fewest-build-containers" choice:"limit-active-tasks" choice:"limit-active-containers" choice:"limit-active-volumes" description:"A third container placement strategy that will only be used for lidar checks."`
	MaxActiveTasksPerWorker      int      `long:"max-active-tasks-per-worker" default:"0" description:"Maximum allowed number of active build tasks per worker. Has effect only when used with limit-active-tasks placement strategy. 0 means no limit."`
	MaxActiveContainersPerWorker int      `long:"max-active-containers-per-worker" default:"0" description:"Maximum allowed number of active containers per worker. Has effect only when used with limit-active-containers placement strategy. 0 means no limit."`
	MaxActiveVolumesPerWorker    int      `long:"max-active-volumes-per-worker" default:"0" description:"Maximum allowed number of active volumes per worker. Has effect only when used with limit-active-volumes placement strategy. 0 means no limit."`
}

var (
	ErrTooManyContainers = errors.New("worker has too many containers")
	ErrTooManyVolumes    = errors.New("worker has too many volumes")
)

func NewPlacementStrategy(options PlacementOptions) (PlacementStrategy, PlacementStrategy, PlacementStrategy, error) {
	// If no-input-container-placement-strategy is not configured, then just use
	// container-placement-strategy.
	if len(options.NoInputStrategies) == 0 {
		options.NoInputStrategies = options.Strategies
	}

	strategy, err := newPlaceStrategy(options, options.Strategies)
	if err != nil {
		return nil, nil, nil, err
	}
	noInputStrategy, err := newPlaceStrategy(options, options.NoInputStrategies)
	if err != nil {
		return nil, nil, nil, err
	}
	checkStrategy, err := newPlaceStrategy(options, options.CheckStrategies)
	if err != nil {
		return nil, nil, nil, err
	}

	return strategy, noInputStrategy, checkStrategy, nil
}

func newPlaceStrategy(options PlacementOptions, chain []string) (PlacementStrategy, error) {
	var strategy PlacementStrategy
	for _, s := range chain {
		switch strings.TrimSpace(s) {
		case "random":
			// Add nothing - since worker order is already randomized
			// initially, a `random` strategy appearing anywhere in the chain
			// of strategies has no effect.
		case "volume-locality":
			strategy = append(strategy, volumeLocalityStrategy{})
		case "fewest-build-containers":
			strategy = append(strategy, fewestBuildContainersStrategy{})
		case "limit-active-tasks":
			if options.MaxActiveTasksPerWorker < 0 {
				return nil, errors.New("max-active-tasks-per-worker must be greater or equal than 0")
			}
			strategy = append(strategy, limitActiveTasksStrategy{MaxTasks: options.MaxActiveTasksPerWorker})
		case "limit-active-containers":
			if options.MaxActiveContainersPerWorker < 0 {
				return nil, errors.New("max-active-containers-per-worker must be greater or equal than 0")
			}
			strategy = append(strategy, limitActiveContainersStrategy{MaxContainers: options.MaxActiveContainersPerWorker})
		case "limit-active-volumes":
			if options.MaxActiveVolumesPerWorker < 0 {
				return nil, errors.New("max-active-volumes-per-worker must be greater or equal than 0")
			}
			strategy = append(strategy, limitActiveVolumesStrategy{MaxVolumes: options.MaxActiveVolumesPerWorker})
		default:
			return nil, fmt.Errorf("invalid container placement strategy %s", strategy)
		}
	}

	return strategy, nil
}

type PlacementStrategy []placementStrategy

type placementStrategy interface {
	// Orders the list of candidate workers based off the configured
	// strategies. Should not remove candidate workers - filtering should
	// be left to Approve.
	Order(lager.Logger, Pool, []db.Worker, runtime.ContainerSpec) ([]db.Worker, error)

	// Attempts to pick the given worker to run the specified container,
	// checking the worker abides by the conditions of the specific strategy.
	Approve(lager.Logger, db.Worker, runtime.ContainerSpec) error

	// Releases any resources acquired by any configured strategies as part of
	// picking the candidate worker.
	Release(lager.Logger, db.Worker, runtime.ContainerSpec)
}

func (strategy PlacementStrategy) Order(logger lager.Logger, pool Pool, workers []db.Worker, spec runtime.ContainerSpec) ([]db.Worker, error) {
	candidates := cloneWorkers(workers)

	// Pre-shuffle the candidate workers to ensure slightly different ordering
	// for workers which are "equal" in the eyes of the configured strategies (eg.
	// have same container counts)
	//
	// Should hopefully prevent a burst of builds from being scheduled on the
	// same worker
	rand.Shuffle(len(candidates), func(i, j int) {
		candidates[i], candidates[j] = candidates[j], candidates[i]
	})

	// We iterate nodes in reverse so the correct ordering is applied
	//
	// For example, if the user specifies "fewest-build-containers,volume-locality" then
	// they should expect candidates to be sorted by those with the fewest build containers,
	// and ties with the number of build containers are broken by the number of volumes
	// which already exists on the worker.
	for i := len(strategy) - 1; i >= 0; i-- {
		var err error
		candidates, err = strategy[i].Order(logger, pool, candidates, spec)
		if err != nil {
			return nil, err
		}
	}

	return candidates, nil
}

func (strategy PlacementStrategy) Approve(logger lager.Logger, worker db.Worker, spec runtime.ContainerSpec) error {
	var err error
	var i int

	for i = 0; i < len(strategy); i++ {
		err = strategy[i].Approve(logger, worker, spec)

		if err != nil {
			// Rollback the stages which successfully passed Approve (i.e. don't include i)
			strategy[:i].Release(logger, worker, spec)
			return err
		}
	}

	return nil
}

func (strategy PlacementStrategy) Release(logger lager.Logger, worker db.Worker, spec runtime.ContainerSpec) {
	for i := len(strategy) - 1; i >= 0; i-- {
		strategy[i].Release(logger, worker, spec)
	}
}

// ------------------------------------------------------
// --------- Individual placement strategies ------------
// ------------------------------------------------------

// volume-locality

type volumeLocalityStrategy struct{}

func (strategy volumeLocalityStrategy) Order(logger lager.Logger, pool Pool, workers []db.Worker, spec runtime.ContainerSpec) ([]db.Worker, error) {
	counts := make(map[string]int, len(workers))
	for _, worker := range workers {
		counts[worker.Name()] = 0
	}

	for _, input := range spec.Inputs {
		if input.FromCache {
			continue
		}

		volume, ok := input.Artifact.(runtime.Volume)
		if !ok {
			// Non-volume artifacts don't live on workers, so don't affect
			// volume locality decisions.
			continue
		}
		logger := logger.WithData(lager.Data{
			"handle": volume.Handle(),
			"path":   input.DestinationPath,
		})
		srcWorker := volume.DBVolume().WorkerName()
		if _, ok := counts[srcWorker]; ok {
			counts[srcWorker]++
		}

		resourceCacheID := volume.DBVolume().GetResourceCacheID()
		if resourceCacheID == 0 {
			logger.Debug("resource-not-cached")
			continue
		}
		resourceCache, found, err := pool.db.ResourceCacheFactory.FindResourceCacheByID(resourceCacheID)
		if err != nil {
			logger.Error("failed-to-find-resource-cache", err)
			return nil, err
		}
		if !found {
			logger.Debug("resource-cache-not-found")
			continue
		}

		workerNames, err := pool.db.VolumeRepo.FindWorkersForResourceCache(resourceCache)
		if err != nil {
			logger.Error("failed-to-find-workers-for-resource-cache", err)
			return nil, err
		}
		for _, worker := range workerNames {
			if worker == srcWorker {
				continue
			}

			if _, ok := counts[worker]; ok {
				counts[worker]++
			}
		}
	}

	for _, cachePath := range spec.Caches {
		logger := logger.WithData(lager.Data{"cache": cachePath})
		usedTaskCache, found, err := pool.db.TaskCacheFactory.Find(spec.JobID, spec.StepName, cachePath)
		if err != nil {
			logger.Error("failed-to-find-task-cache", err)
			return nil, err
		}
		if !found {
			logger.Debug("task-cache-not-found")
			continue
		}

		workerNames, err := pool.db.VolumeRepo.FindWorkersForTaskCache(usedTaskCache)
		if err != nil {
			logger.Error("failed-to-find-workers-for-task-cache", err)
			return nil, err
		}
		for _, worker := range workerNames {
			if _, ok := counts[worker]; ok {
				counts[worker]++
			}
		}
	}

	sortedWorkers := cloneWorkers(workers)
	sort.SliceStable(sortedWorkers, func(i, j int) bool {
		return counts[sortedWorkers[i].Name()] > counts[sortedWorkers[j].Name()]
	})

	return sortedWorkers, nil
}

func (volumeLocalityStrategy) Approve(lager.Logger, db.Worker, runtime.ContainerSpec) error {
	return nil
}

func (volumeLocalityStrategy) Release(lager.Logger, db.Worker, runtime.ContainerSpec) {}

// fewest-build-containers

type fewestBuildContainersStrategy struct{}

func (strategy fewestBuildContainersStrategy) Order(logger lager.Logger, pool Pool, workers []db.Worker, spec runtime.ContainerSpec) ([]db.Worker, error) {
	counts, err := pool.db.WorkerFactory.BuildContainersCountPerWorker()
	if err != nil {
		return nil, err
	}

	sortedWorkers := cloneWorkers(workers)
	sort.SliceStable(sortedWorkers, func(i, j int) bool {
		return counts[sortedWorkers[i].Name()] < counts[sortedWorkers[j].Name()]
	})

	return sortedWorkers, nil
}

func (fewestBuildContainersStrategy) Approve(lager.Logger, db.Worker, runtime.ContainerSpec) error {
	return nil
}

func (fewestBuildContainersStrategy) Release(lager.Logger, db.Worker, runtime.ContainerSpec) {}

// limit-active-tasks

type limitActiveTasksStrategy struct {
	MaxTasks int
}

func (strategy limitActiveTasksStrategy) Order(logger lager.Logger, pool Pool, workers []db.Worker, spec runtime.ContainerSpec) ([]db.Worker, error) {
	if spec.Type != db.ContainerTypeTask {
		return workers, nil
	}

	taskCounts := make(map[db.Worker]int, len(workers))
	candidates := make([]db.Worker, 0, len(workers))

	for _, worker := range workers {
		logger := logger.WithData(lager.Data{"worker": worker.Name()})
		activeTasks, err := worker.ActiveTasks()
		if err != nil {
			// just skip this worker
			logger.Error("retrieve-active-tasks-on-worker", err)
			continue
		}

		candidates = append(candidates, worker)
		taskCounts[worker] = activeTasks
	}

	sort.SliceStable(candidates, func(i, j int) bool {
		return taskCounts[candidates[i]] < taskCounts[candidates[j]]
	})

	return candidates, nil
}

func (strategy limitActiveTasksStrategy) Approve(logger lager.Logger, worker db.Worker, spec runtime.ContainerSpec) error {
	if spec.Type != db.ContainerTypeTask || strategy.MaxTasks == 0 {
		return nil
	}

	_, err := worker.IncreaseActiveTasks(strategy.MaxTasks)

	return err
}

func (strategy limitActiveTasksStrategy) Release(logger lager.Logger, worker db.Worker, spec runtime.ContainerSpec) {
	if spec.Type != db.ContainerTypeTask {
		return
	}

	_, err := worker.DecreaseActiveTasks()
	if err != nil {
		logger.Error("failed-to-decrease-active-tasks", err)
	}
}

// limit-active-containers

type limitActiveContainersStrategy struct {
	MaxContainers int
}

func (strategy limitActiveContainersStrategy) Order(logger lager.Logger, pool Pool, workers []db.Worker, spec runtime.ContainerSpec) ([]db.Worker, error) {
	return partitionWorkersBy(workers, strategy.workerSatisfies), nil
}

func (strategy limitActiveContainersStrategy) workerSatisfies(worker db.Worker) bool {
	if strategy.MaxContainers == 0 {
		return true
	}

	return worker.ActiveContainers() < strategy.MaxContainers
}

func (strategy limitActiveContainersStrategy) Approve(_ lager.Logger, worker db.Worker, _ runtime.ContainerSpec) error {
	if !strategy.workerSatisfies(worker) {
		return ErrTooManyContainers
	}

	return nil
}

func (strategy limitActiveContainersStrategy) Release(lager.Logger, db.Worker, runtime.ContainerSpec) {
}

// limit-active-volumes

type limitActiveVolumesStrategy struct {
	MaxVolumes int
}

func (strategy limitActiveVolumesStrategy) Order(logger lager.Logger, pool Pool, workers []db.Worker, spec runtime.ContainerSpec) ([]db.Worker, error) {
	return partitionWorkersBy(workers, strategy.workerSatisfies), nil
}

func (strategy limitActiveVolumesStrategy) workerSatisfies(worker db.Worker) bool {
	if strategy.MaxVolumes == 0 {
		return true
	}

	return worker.ActiveVolumes() < strategy.MaxVolumes
}

func (strategy limitActiveVolumesStrategy) Approve(_ lager.Logger, worker db.Worker, _ runtime.ContainerSpec) error {
	if !strategy.workerSatisfies(worker) {
		return ErrTooManyVolumes
	}

	return nil
}

func (strategy limitActiveVolumesStrategy) Release(lager.Logger, db.Worker, runtime.ContainerSpec) {
}

// helpers

func cloneWorkers(workers []db.Worker) []db.Worker {
	clone := make([]db.Worker, len(workers))
	copy(clone, workers)
	return clone
}

func partitionWorkersBy(workers []db.Worker, pred func(db.Worker) bool) []db.Worker {
	partitionGroup := func(worker db.Worker) int {
		if pred(worker) {
			return 0
		} else {
			return 1
		}
	}

	sorted := cloneWorkers(workers)
	sort.SliceStable(sorted, func(i, j int) bool {
		return partitionGroup(sorted[i]) < partitionGroup(sorted[j])
	})

	return sorted
}
