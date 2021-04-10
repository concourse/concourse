package worker

import (
	"errors"
	"fmt"
	"math/rand"
	"sort"
	"strings"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db"
)

type ContainerPlacementStrategyOptions struct {
	ContainerPlacementStrategy   []string `yaml:"container_placement_strategy,omitempty" validate:"dive,cps"`
	MaxActiveTasksPerWorker      int      `yaml:"max_active_tasks_per_worker,omitempty"`
	MaxActiveContainersPerWorker int      `yaml:"max_active_containers_per_worker,omitempty"`
	MaxActiveVolumesPerWorker    int      `yaml:"max_active_volumes_per_worker,omitempty"`
}

var (
	ErrTooManyActiveTasks = errors.New("worker has too many active tasks")
	ErrTooManyContainers  = errors.New("worker has too many containers")
	ErrTooManyVolumes     = errors.New("worker has too many volumes")
)

type NoWorkerFitContainerPlacementStrategyError struct {
	Strategy string
}

func (err NoWorkerFitContainerPlacementStrategyError) Error() string {
	return fmt.Sprintf("no worker fit container placement strategy: %s", err.Strategy)
}

type ContainerPlacementStrategy interface {
	// TODO: Don't pass around container metadata since it's not guaranteed to be deterministic.
	// Change this after check containers stop being reused

	Name() string

	// Orders the list of candidate workers based off the configured strategies. Should only remove
	// candidate workers which will never match the strategy. Filtering should mostly be left to Pick.
	Order(lager.Logger, []Worker, ContainerSpec) ([]Worker, error)

	// Attempts to pick the given worker to run the specified container, checking the worker abides
	// by the conditions of the specific strategy.
	Pick(lager.Logger, Worker, ContainerSpec) error

	// Releases any resources acquired by any configured strategies as part of
	// picking the candidate worker.
	Release(lager.Logger, Worker, ContainerSpec)
}

type ChainPlacementStrategy struct {
	nodes []ContainerPlacementStrategy
}

func NewRandomPlacementStrategy() ContainerPlacementStrategy {
	s, _ := NewChainPlacementStrategy(ContainerPlacementStrategyOptions{ContainerPlacementStrategy: []string{"random"}})
	return s
}

func NewChainPlacementStrategy(opts ContainerPlacementStrategyOptions) (*ChainPlacementStrategy, error) {
	cps := &ChainPlacementStrategy{
		nodes: []ContainerPlacementStrategy{},
	}

	for _, strategy := range opts.ContainerPlacementStrategy {
		strategy := strings.TrimSpace(strategy)
		switch atc.ContainerPlacementStrategy(strategy) {
		case atc.ContainerPlacementRandom:
			// Add nothing. Because an empty strategy chain equals to random strategy.
		case atc.ContainerPlacementFewestBuildContainers:
			cps.nodes = append(cps.nodes, newFewestBuildContainersStrategy(strategy))

		case atc.ContainerPlacementLimitActiveTasks:
			if opts.MaxActiveTasksPerWorker < 0 {
				return nil, errors.New("max-active-tasks-per-worker must be greater or equal than 0")
			}
			cps.nodes = append(cps.nodes, newLimitActiveTasksStrategy(strategy, opts.MaxActiveTasksPerWorker))

		case atc.ContainerPlacementLimitActiveContainers:
			if opts.MaxActiveContainersPerWorker < 0 {
				return nil, errors.New("max-active-containers-per-worker must be greater or equal than 0")
			}
			cps.nodes = append(cps.nodes, newLimitActiveContainersStrategy(strategy, opts.MaxActiveContainersPerWorker))

		case atc.ContainerPlacementLimitActiveVolumes:
			if opts.MaxActiveVolumesPerWorker < 0 {
				return nil, errors.New("max-active-volumes-per-worker must be greater or equal than 0")
			}
			cps.nodes = append(cps.nodes, newLimitActiveVolumesPlacementStrategy(strategy, opts.MaxActiveVolumesPerWorker))

		case atc.ContainerPlacementVolumeLocality:
			cps.nodes = append(cps.nodes, newVolumeLocalityStrategy(strategy))

		default:
			return nil, fmt.Errorf("invalid container placement strategy %s", strategy)
		}
	}

	return cps, nil
}

func (strategy *ChainPlacementStrategy) Name() string {
	names := []string{}
	for _, node := range strategy.nodes {
		names = append(names, node.Name())
	}

	return strings.Join(names, ",")
}

func (strategy *ChainPlacementStrategy) Order(logger lager.Logger, workers []Worker, spec ContainerSpec) ([]Worker, error) {
	candidates := append([]Worker(nil), workers...)

	// Pre-shuffle the candidate workers to ensure slightly different ordering
	// for workers which are "equal" in the eyes of the configured strategies (eg.
	// have same container counts)
	//
	// Should hopefully prevent a burst of builds from being scheduled on the
	// same worker
	rand.Shuffle(len(candidates), func(i, j int) {
		candidates[i], candidates[j] = candidates[j], candidates[i]
	})

	// We iterate nodes in reverse so the correct ordering is applies
	//
	// For example, if the user specifies "fewest-build-containers,volume-locality" then
	// they should expect candidates to be sorted by those with the fewest build containers,
	// and ties with the number of build containers are broken by the number of volumes
	// which already exists on the worker.
	for i := len(strategy.nodes) - 1; i >= 0; i-- {
		node := strategy.nodes[i]

		var err error
		candidates, err = node.Order(logger, candidates, spec)
		if err != nil {
			return nil, err
		}

		if len(candidates) == 0 {
			return nil, NoWorkerFitContainerPlacementStrategyError{Strategy: node.Name()}
		}
	}

	return candidates, nil
}

func (strategy *ChainPlacementStrategy) Pick(logger lager.Logger, worker Worker, spec ContainerSpec) error {
	var err error
	var i int

	// Use "i" from the function scope so we can call rollback and call
	// Release on the relevant nodes when an error occurs.
	for i = 0; i < len(strategy.nodes); i++ {
		node := strategy.nodes[i]
		err = node.Pick(logger, worker, spec)

		if err != nil {
			break
		}
	}

	if err != nil {
		// On error, call Release on all stages which successfully passed
		// Pick. Decrement "i" initially to skip stage which failed Pick.
		for i--; i >= 0; i-- {
			node := strategy.nodes[i]
			node.Release(logger, worker, spec)
		}
	}

	return err
}

func (strategy *ChainPlacementStrategy) Release(logger lager.Logger, worker Worker, spec ContainerSpec) {
	for i := len(strategy.nodes) - 1; i >= 0; i-- {
		node := strategy.nodes[i]
		node.Release(logger, worker, spec)
	}
}

type NamedPlacementStrategy struct {
	name string
}

func (strategy *NamedPlacementStrategy) Name() string {
	return strategy.name
}

// Strategy which orders candidate workers based off the number of volumes which already
// exist on them
type VolumeLocalityStrategy struct {
	NamedPlacementStrategy
}

func newVolumeLocalityStrategy(name string) ContainerPlacementStrategy {
	return &VolumeLocalityStrategy{
		NamedPlacementStrategy{name},
	}
}

func (strategy *VolumeLocalityStrategy) Order(logger lager.Logger, workers []Worker, spec ContainerSpec) ([]Worker, error) {
	candidates := append([]Worker(nil), workers...)
	counts := make(map[Worker]int, len(candidates))

	for _, worker := range workers {
		inputCount := 0

		for _, inputSource := range spec.Inputs {
			_, found, err := inputSource.Source().ExistsOn(logger, worker)

			if err != nil {
				return nil, err
			}

			if found {
				inputCount++
			}
		}

		counts[worker] = inputCount
	}

	sort.SliceStable(candidates, func(i, j int) bool {
		return counts[candidates[i]] > counts[candidates[j]]
	})

	return candidates, nil
}

func (strategy *VolumeLocalityStrategy) Pick(logger lager.Logger, worker Worker, spec ContainerSpec) error {
	// This strategy doesn't have any requirements on the number of volumes which must exist
	// on a worker for the container to be scheduled on it
	return nil
}

func (strategy *VolumeLocalityStrategy) Release(logger lager.Logger, worker Worker, spec ContainerSpec) {
}

// Strategy which orders candidate workers based off the number of build containers which
// are already running on them
type FewestBuildContainersStrategy struct {
	NamedPlacementStrategy
}

func newFewestBuildContainersStrategy(name string) ContainerPlacementStrategy {
	return &FewestBuildContainersStrategy{
		NamedPlacementStrategy{name},
	}
}

func (strategy *FewestBuildContainersStrategy) Order(logger lager.Logger, workers []Worker, spec ContainerSpec) ([]Worker, error) {
	candidates := append([]Worker(nil), workers...)
	counts := make(map[Worker]int, len(candidates))

	for _, worker := range workers {
		counts[worker] = worker.BuildContainers()
	}

	sort.SliceStable(candidates, func(i, j int) bool {
		return counts[candidates[i]] < counts[candidates[j]]
	})

	return candidates, nil
}

func (strategy *FewestBuildContainersStrategy) Pick(logger lager.Logger, worker Worker, spec ContainerSpec) error {
	return nil
}

func (strategy *FewestBuildContainersStrategy) Release(logger lager.Logger, worker Worker, spec ContainerSpec) {
}

type LimitActiveTasksStrategy struct {
	NamedPlacementStrategy
	maxTasks int
}

func newLimitActiveTasksStrategy(name string, maxTasks int) ContainerPlacementStrategy {
	return &LimitActiveTasksStrategy{
		NamedPlacementStrategy: NamedPlacementStrategy{name},
		maxTasks:               maxTasks,
	}
}

func (strategy *LimitActiveTasksStrategy) Order(logger lager.Logger, workers []Worker, spec ContainerSpec) ([]Worker, error) {
	if spec.Type != db.ContainerTypeTask {
		return workers, nil
	}

	candidates := []Worker{}
	taskCounts := map[Worker]int{}

	for _, worker := range workers {
		activeTasks, err := worker.ActiveTasks()

		if err != nil {
			logger.Error("Cannot retrieve active tasks on worker. Skipping.", err)
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

func (strategy *LimitActiveTasksStrategy) Pick(logger lager.Logger, worker Worker, spec ContainerSpec) error {
	if spec.Type != db.ContainerTypeTask {
		return nil
	}

	activeTasks, err := worker.IncreaseActiveTasks()

	if err != nil {
		return err
	}

	if strategy.maxTasks > 0 && activeTasks > strategy.maxTasks {
		_, err := worker.DecreaseActiveTasks()
		if err != nil {
			logger.Error("failed-to-decrease-active-tasks", err)
		}

		return ErrTooManyActiveTasks
	}

	return nil
}

func (strategy *LimitActiveTasksStrategy) Release(logger lager.Logger, worker Worker, spec ContainerSpec) {
	if spec.Type != db.ContainerTypeTask {
		return
	}

	_, err := worker.DecreaseActiveTasks()
	if err != nil {
		logger.Error("failed-to-decrease-active-tasks", err)
	}
}

type LimitActiveContainersStrategy struct {
	NamedPlacementStrategy
	maxContainers int
}

func newLimitActiveContainersStrategy(name string, maxContainers int) ContainerPlacementStrategy {
	return &LimitActiveContainersStrategy{
		NamedPlacementStrategy: NamedPlacementStrategy{name},
		maxContainers:          maxContainers,
	}
}

func (strategy *LimitActiveContainersStrategy) Order(logger lager.Logger, workers []Worker, spec ContainerSpec) ([]Worker, error) {
	return workers, nil
}

func (strategy *LimitActiveContainersStrategy) Pick(logger lager.Logger, worker Worker, spec ContainerSpec) error {
	if strategy.maxContainers > 0 && worker.ActiveContainers() > strategy.maxContainers {
		return ErrTooManyContainers
	}

	return nil
}

func (strategy *LimitActiveContainersStrategy) Release(logger lager.Logger, worker Worker, spec ContainerSpec) {
}

type LimitActiveVolumesStrategy struct {
	NamedPlacementStrategy
	maxVolumes int
}

func newLimitActiveVolumesPlacementStrategy(name string, maxVolumes int) ContainerPlacementStrategy {
	return &LimitActiveVolumesStrategy{
		NamedPlacementStrategy: NamedPlacementStrategy{name},
		maxVolumes:             maxVolumes,
	}
}

func (strategy *LimitActiveVolumesStrategy) Order(logger lager.Logger, workers []Worker, spec ContainerSpec) ([]Worker, error) {
	return workers, nil
}

func (strategy *LimitActiveVolumesStrategy) Pick(logger lager.Logger, worker Worker, spec ContainerSpec) error {
	if strategy.maxVolumes > 0 && worker.ActiveVolumes() > strategy.maxVolumes {
		return ErrTooManyVolumes
	}

	return nil
}

func (strategy *LimitActiveVolumesStrategy) Release(logger lager.Logger, worker Worker, spec ContainerSpec) {
}
