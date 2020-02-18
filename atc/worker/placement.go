package worker

import (
	"math/rand"
	"time"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/concourse/atc/db"
)

type ContainerPlacementStrategy interface {
	//TODO: Don't pass around container metadata since it's not guaranteed to be deterministic.
	// Change this after check containers stop being reused
	Choose(lager.Logger, []Worker, ContainerSpec) (Worker, error)
	ModifiesActiveTasks() bool
}

type VolumeLocalityPlacementStrategy struct {
	rand *rand.Rand
}

func NewVolumeLocalityPlacementStrategy() ContainerPlacementStrategy {
	return &VolumeLocalityPlacementStrategy{
		rand: rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

func (strategy *VolumeLocalityPlacementStrategy) Choose(logger lager.Logger, workers []Worker, spec ContainerSpec) (Worker, error) {
	workersByCount := map[int][]Worker{}
	var highestCount int
	for _, w := range workers {
		candidateInputCount := 0

		for _, inputSource := range spec.Inputs {
			_, found, err := inputSource.Source().ExistsOn(logger, w)
			if err != nil {
				return nil, err
			}

			if found {
				candidateInputCount++
			}
		}

		workersByCount[candidateInputCount] = append(workersByCount[candidateInputCount], w)

		if candidateInputCount >= highestCount {
			highestCount = candidateInputCount
		}
	}

	highestLocalityWorkers := workersByCount[highestCount]

	return highestLocalityWorkers[strategy.rand.Intn(len(highestLocalityWorkers))], nil
}

func (strategy *VolumeLocalityPlacementStrategy) ModifiesActiveTasks() bool {
	return false
}

type FewestBuildContainersPlacementStrategy struct {
	rand *rand.Rand
}

func NewFewestBuildContainersPlacementStrategy() ContainerPlacementStrategy {
	return &FewestBuildContainersPlacementStrategy{
		rand: rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

func (strategy *FewestBuildContainersPlacementStrategy) Choose(logger lager.Logger, workers []Worker, spec ContainerSpec) (Worker, error) {
	workersByWork := map[int][]Worker{}
	var minWork int

	for i, w := range workers {
		work := w.BuildContainers()
		workersByWork[work] = append(workersByWork[work], w)
		if i == 0 || work < minWork {
			minWork = work
		}
	}

	leastBusyWorkers := workersByWork[minWork]
	return leastBusyWorkers[strategy.rand.Intn(len(leastBusyWorkers))], nil
}

func (strategy *FewestBuildContainersPlacementStrategy) ModifiesActiveTasks() bool {
	return false
}

type LimitActiveTasksPlacementStrategy struct {
	rand     *rand.Rand
	maxTasks int
}

func NewLimitActiveTasksPlacementStrategy(maxTasks int) ContainerPlacementStrategy {
	return &LimitActiveTasksPlacementStrategy{
		rand:     rand.New(rand.NewSource(time.Now().UnixNano())),
		maxTasks: maxTasks,
	}
}

func (strategy *LimitActiveTasksPlacementStrategy) Choose(logger lager.Logger, workers []Worker, spec ContainerSpec) (Worker, error) {
	workersByWork := map[int][]Worker{}
	minActiveTasks := -1

	for _, w := range workers {
		activeTasks, err := w.ActiveTasks()
		if err != nil {
			logger.Error("Cannot retrive active tasks on worker. Skipping.", err)
			continue
		}

		// If maxTasks == 0 or the step is not a task, ignore the number of active tasks and distribute the work evenly
		if strategy.maxTasks > 0 && activeTasks >= strategy.maxTasks && spec.Type == db.ContainerTypeTask {
			logger.Info("worker-busy")
			continue
		}

		workersByWork[activeTasks] = append(workersByWork[activeTasks], w)
		if minActiveTasks == -1 || activeTasks < minActiveTasks {
			minActiveTasks = activeTasks
		}
	}

	leastBusyWorkers := workersByWork[minActiveTasks]
	if len(leastBusyWorkers) < 1 {
		return nil, nil
	}
	return leastBusyWorkers[strategy.rand.Intn(len(leastBusyWorkers))], nil
}

func (strategy *LimitActiveTasksPlacementStrategy) ModifiesActiveTasks() bool {
	return true
}

type RandomPlacementStrategy struct {
	rand *rand.Rand
}

func NewRandomPlacementStrategy() ContainerPlacementStrategy {
	return &RandomPlacementStrategy{
		rand: rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

func (strategy *RandomPlacementStrategy) Choose(logger lager.Logger, workers []Worker, spec ContainerSpec) (Worker, error) {
	return workers[strategy.rand.Intn(len(workers))], nil
}

func (strategy *RandomPlacementStrategy) ModifiesActiveTasks() bool {
	return false
}
