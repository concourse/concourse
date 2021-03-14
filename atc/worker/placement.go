package worker

import (
	"errors"
	"fmt"
	"math/rand"
	"strings"
	"time"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/runtime"
)

type PlacementOptions struct {
	Strategies                   []string `long:"container-placement-strategy" default:"volume-locality" choice:"volume-locality" choice:"random" choice:"fewest-build-containers" choice:"limit-active-tasks" choice:"limit-active-containers" choice:"limit-active-volumes" description:"Method by which a worker is selected during container placement. If multiple methods are specified, they will be applied in order. Random strategy should only be used alone."`
	MaxActiveTasksPerWorker      int      `long:"max-active-tasks-per-worker" default:"0" description:"Maximum allowed number of active build tasks per worker. Has effect only when used with limit-active-tasks placement strategy. 0 means no limit."`
	MaxActiveContainersPerWorker int      `long:"max-active-containers-per-worker" default:"0" description:"Maximum allowed number of active containers per worker. Has effect only when used with limit-active-containers placement strategy. 0 means no limit."`
	MaxActiveVolumesPerWorker    int      `long:"max-active-volumes-per-worker" default:"0" description:"Maximum allowed number of active volumes per worker. Has effect only when used with limit-active-volumes placement strategy. 0 means no limit."`
}

func NewPlacementStrategy(options PlacementOptions) (PlacementStrategy, error) {
	var strategy PlacementStrategy
	for _, s := range options.Strategies {
		switch strings.TrimSpace(s) {
		case "random":
			// Add nothing. Because an empty strategy chain equals to random strategy.
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

func (strategy PlacementStrategy) Choose(logger lager.Logger, pool Pool, workers []db.Worker, spec runtime.ContainerSpec) (db.Worker, error) {
	for _, s := range strategy {
		var err error
		workers, err = s.Filter(logger, pool, workers, spec)
		if err != nil {
			return nil, err
		}
		if len(workers) == 0 {
			return nil, NoWorkerFitContainerPlacementStrategyError{Strategy: s.Name()}
		}
	}
	if len(workers) == 1 {
		return workers[0], nil
	}

	// If there are still multiple candidate, choose a random one.
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	return workers[r.Intn(len(workers))], nil
}

func (strategy PlacementStrategy) ModifiesActiveTasks() bool {
	for _, s := range strategy {
		if _, ok := s.(limitActiveTasksStrategy); ok {
			return true
		}
	}
	return false
}

type placementStrategy interface {
	Name() string
	Filter(lager.Logger, Pool, []db.Worker, runtime.ContainerSpec) ([]db.Worker, error)
}

type volumeLocalityStrategy struct{}

func (volumeLocalityStrategy) Name() string { return "volume-locality" }

func (strategy volumeLocalityStrategy) Filter(logger lager.Logger, pool Pool, workers []db.Worker, spec runtime.ContainerSpec) ([]db.Worker, error) {
	workerByName := make(map[string]db.Worker, len(workers))
	countByWorker := make(map[string]int, len(workers))
	for _, worker := range workers {
		workerByName[worker.Name()] = worker
		countByWorker[worker.Name()] = 0
	}

	var highestCount int
	increment := func(workerName string) {
		curCount, ok := countByWorker[workerName]
		if !ok {
			// only consider workers that aren't yet filtered out
			return
		}
		newCount := curCount + 1
		countByWorker[workerName] = newCount
		if newCount > highestCount {
			highestCount = newCount
		}
	}
	for _, input := range spec.Inputs {
		logger := logger.WithData(lager.Data{
			"handle": input.VolumeHandle,
			"path":   input.DestinationPath,
		})
		volume, srcWorker, found, err := pool.LocateVolume(logger, spec.TeamID, input.VolumeHandle)
		if err != nil {
			logger.Error("failed-to-locate-volume", err)
			return nil, err
		}
		if !found {
			logger.Info("input-volume-not-found")
			continue
		}
		increment(srcWorker.Name())

		resourceCacheID := volume.DBVolume().GetResourceCacheID()
		if resourceCacheID == 0 {
			logger.Debug("resource-not-cached")
			continue
		}
		resourceCache, found, err := pool.DB.ResourceCacheFactory.FindResourceCacheByID(resourceCacheID)
		if err != nil {
			logger.Error("failed-to-find-resource-cache", err)
			return nil, err
		}
		if !found {
			logger.Debug("resource-cache-not-found")
			continue
		}
		for _, worker := range workers {
			if worker.Name() == srcWorker.Name() {
				continue
			}
			_, found, err := pool.DB.VolumeRepo.FindResourceCacheVolume(worker.Name(), resourceCache)
			if err != nil {
				logger.Error("failed-to-find-resource-cache-volume", err)
				return nil, err
			}
			if found {
				increment(worker.Name())
			}
		}
	}

	for _, cachePath := range spec.Caches {
		logger := logger.WithData(lager.Data{"cache": cachePath})
		usedTaskCache, found, err := pool.DB.TaskCacheFactory.Find(spec.JobID, spec.StepName, cachePath)
		if err != nil {
			logger.Error("failed-to-find-task-cache", err)
			return nil, err
		}
		if !found {
			logger.Debug("task-cache-not-found")
			continue
		}

		for _, worker := range workers {
			_, found, err := pool.DB.VolumeRepo.FindTaskCacheVolume(spec.TeamID, worker.Name(), usedTaskCache)
			if err != nil {
				logger.Error("failed-to-find-task-cache-volume", err)
				return nil, err
			}
			if found {
				increment(worker.Name())
			}
		}
	}

	if highestCount == 0 {
		return workers, nil
	}

	var optimalWorkers []db.Worker
	for worker, count := range countByWorker {
		if count == highestCount {
			optimalWorkers = append(optimalWorkers, workerByName[worker])
		}
	}

	return optimalWorkers, nil
}

type fewestBuildContainersStrategy struct{}

func (fewestBuildContainersStrategy) Name() string { return "fewest-build-containers" }

func (strategy fewestBuildContainersStrategy) Filter(logger lager.Logger, pool Pool, workers []db.Worker, spec runtime.ContainerSpec) ([]db.Worker, error) {
	workersByWork := map[int][]db.Worker{}
	var minWork int

	for i, w := range workers {
		work := w.ActiveContainers()
		workersByWork[work] = append(workersByWork[work], w)
		if i == 0 || work < minWork {
			minWork = work
		}
	}

	return workersByWork[minWork], nil
}

type limitActiveTasksStrategy struct {
	MaxTasks int
}

func (limitActiveTasksStrategy) Name() string { return "limit-active-tasks" }

func (strategy limitActiveTasksStrategy) Filter(logger lager.Logger, pool Pool, workers []db.Worker, spec runtime.ContainerSpec) ([]db.Worker, error) {
	workersByWork := map[int][]db.Worker{}
	var minActiveTasks int

	for i, w := range workers {
		logger := logger.WithData(lager.Data{"worker": w.Name()})
		activeTasks, err := w.ActiveTasks()
		if err != nil {
			logger.Error("retrieve-active-tasks-on-worker", err)
			continue
		}

		// If MaxTasks == 0 or the step is not a task, ignore the number of active tasks and distribute the work evenly
		if strategy.MaxTasks > 0 && activeTasks >= strategy.MaxTasks && spec.Type == db.ContainerTypeTask {
			logger.Info("worker-busy")
			continue
		}

		workersByWork[activeTasks] = append(workersByWork[activeTasks], w)
		if i == 0 || activeTasks < minActiveTasks {
			minActiveTasks = activeTasks
		}
	}

	return workersByWork[minActiveTasks], nil
}

type limitActiveContainersStrategy struct {
	MaxContainers int
}

func (limitActiveContainersStrategy) Name() string { return "limit-active-containers" }

func (strategy limitActiveContainersStrategy) Filter(logger lager.Logger, pool Pool, workers []db.Worker, spec runtime.ContainerSpec) ([]db.Worker, error) {
	if strategy.MaxContainers == 0 {
		return workers, nil
	}

	candidates := []db.Worker{}
	for _, w := range workers {
		if w.ActiveContainers() < strategy.MaxContainers {
			candidates = append(candidates, w)
		}
	}
	return candidates, nil
}

type limitActiveVolumesStrategy struct {
	MaxVolumes int
}

func (limitActiveVolumesStrategy) Name() string { return "limit-active-volumes" }

func (strategy limitActiveVolumesStrategy) Filter(logger lager.Logger, pool Pool, workers []db.Worker, spec runtime.ContainerSpec) ([]db.Worker, error) {
	if strategy.MaxVolumes == 0 {
		return workers, nil
	}

	candidates := []db.Worker{}
	for _, w := range workers {
		if w.ActiveVolumes() < strategy.MaxVolumes {
			candidates = append(candidates, w)
		}
	}
	return candidates, nil
}
