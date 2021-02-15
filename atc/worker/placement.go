package worker

import (
	"errors"
	"fmt"
	"math/rand"
	"strings"
	"time"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/concourse/atc/db"
)

type ContainerPlacementStrategyOptions struct {
	ContainerPlacementStrategy   []string `yaml:"container_placement_strategy" validate:"cps"`
	MaxActiveTasksPerWorker      int      `yaml:"max_active_tasks_per_worker"`
	MaxActiveContainersPerWorker int      `yaml:"max_active_containers_per_worker"`
	MaxActiveVolumesPerWorker    int      `yaml:"max_active_volumes_per_worker"`
}

type NoWorkerFitContainerPlacementStrategyError struct {
	Strategy string
}

func (err NoWorkerFitContainerPlacementStrategyError) Error() string {
	return fmt.Sprintf("no worker fit container placement strategy: %s", err.Strategy)
}

type ContainerPlacementStrategy interface {
	//TODO: Don't pass around container metadata since it's not guaranteed to be deterministic.
	// Change this after check containers stop being reused
	Choose(lager.Logger, []Worker, ContainerSpec) (Worker, error)
	ModifiesActiveTasks() bool
}

type ContainerPlacementStrategyChainNode interface {
	Choose(lager.Logger, []Worker, ContainerSpec) ([]Worker, error)
	ModifiesActiveTasks() bool
	StrategyName() string
}

type containerPlacementStrategy struct {
	nodes []ContainerPlacementStrategyChainNode
}

func NewContainerPlacementStrategy(opts ContainerPlacementStrategyOptions) (*containerPlacementStrategy, error) {
	cps := &containerPlacementStrategy{nodes: []ContainerPlacementStrategyChainNode{}}
	for _, strategy := range opts.ContainerPlacementStrategy {
		strategy := strings.TrimSpace(strategy)
		switch strategy {
		case "random":
			// Add nothing. Because an empty strategy chain equals to random strategy.
		case "fewest-build-containers":
			cps.nodes = append(cps.nodes, newFewestBuildContainersPlacementStrategy(strategy))
		case "limit-active-tasks":
			if opts.MaxActiveTasksPerWorker < 0 {
				return nil, errors.New("max-active-tasks-per-worker must be greater or equal than 0")
			}
			cps.nodes = append(cps.nodes, newLimitActiveTasksPlacementStrategy(strategy, opts.MaxActiveTasksPerWorker))
		case "limit-active-containers":
			if opts.MaxActiveContainersPerWorker < 0 {
				return nil, errors.New("max-active-containers-per-worker must be greater or equal than 0")
			}
			cps.nodes = append(cps.nodes, newLimitActiveContainersPlacementStrategy(strategy, opts.MaxActiveContainersPerWorker))
		case "limit-active-volumes":
			if opts.MaxActiveVolumesPerWorker < 0 {
				return nil, errors.New("max-active-volumes-per-worker must be greater or equal than 0")
			}
			cps.nodes = append(cps.nodes, newLimitActiveVolumesPlacementStrategy(strategy, opts.MaxActiveVolumesPerWorker))
		case "volume-locality":
			cps.nodes = append(cps.nodes, newVolumeLocalityPlacementStrategyNode(strategy))
		default:
			return nil, fmt.Errorf("invalid container placement strategy %s", strategy)
		}
	}
	return cps, nil
}

func (strategy *containerPlacementStrategy) Choose(logger lager.Logger, workers []Worker, spec ContainerSpec) (Worker, error) {
	var err error
	for _, node := range strategy.nodes {
		workers, err = node.Choose(logger, workers, spec)
		if err != nil {
			return nil, err
		}
		if len(workers) == 0 {
			return nil, NoWorkerFitContainerPlacementStrategyError{Strategy: node.StrategyName()}
		}
	}
	if len(workers) == 1 {
		return workers[0], nil
	}

	// If there are still multiple candidate, choose a random one.
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	return workers[r.Intn(len(workers))], nil
}

func (strategy *containerPlacementStrategy) ModifiesActiveTasks() bool {
	for _, node := range strategy.nodes {
		if node.ModifiesActiveTasks() {
			return true
		}
	}
	return false
}

func NewRandomPlacementStrategy() ContainerPlacementStrategy {
	s, _ := NewContainerPlacementStrategy(ContainerPlacementStrategyOptions{ContainerPlacementStrategy: []string{"random"}})
	return s
}

type VolumeLocalityPlacementStrategyNode struct {
	GivenName string
}

func newVolumeLocalityPlacementStrategyNode(name string) ContainerPlacementStrategyChainNode {
	return &VolumeLocalityPlacementStrategyNode{name}
}

func (strategy *VolumeLocalityPlacementStrategyNode) Choose(logger lager.Logger, workers []Worker, spec ContainerSpec) ([]Worker, error) {
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

	return workersByCount[highestCount], nil
}

func (strategy *VolumeLocalityPlacementStrategyNode) ModifiesActiveTasks() bool {
	return false
}

func (strategy *VolumeLocalityPlacementStrategyNode) StrategyName() string {
	return strategy.GivenName
}

type FewestBuildContainersPlacementStrategyNode struct {
	GivenName string
}

func newFewestBuildContainersPlacementStrategy(name string) ContainerPlacementStrategyChainNode {
	return &FewestBuildContainersPlacementStrategyNode{name}
}

func (strategy *FewestBuildContainersPlacementStrategyNode) Choose(logger lager.Logger, workers []Worker, spec ContainerSpec) ([]Worker, error) {
	workersByWork := map[int][]Worker{}
	var minWork int

	for i, w := range workers {
		work := w.BuildContainers()
		workersByWork[work] = append(workersByWork[work], w)
		if i == 0 || work < minWork {
			minWork = work
		}
	}

	return workersByWork[minWork], nil
}

func (strategy *FewestBuildContainersPlacementStrategyNode) ModifiesActiveTasks() bool {
	return false
}

func (strategy *FewestBuildContainersPlacementStrategyNode) StrategyName() string {
	return strategy.GivenName
}

type LimitActiveTasksPlacementStrategyNode struct {
	GivenName string
	maxTasks  int
}

func newLimitActiveTasksPlacementStrategy(name string, maxTasks int) ContainerPlacementStrategyChainNode {
	return &LimitActiveTasksPlacementStrategyNode{
		GivenName: name,
		maxTasks:  maxTasks,
	}
}

func (strategy *LimitActiveTasksPlacementStrategyNode) Choose(logger lager.Logger, workers []Worker, spec ContainerSpec) ([]Worker, error) {
	workersByWork := map[int][]Worker{}
	minActiveTasks := -1

	for _, w := range workers {
		activeTasks, err := w.ActiveTasks()
		if err != nil {
			logger.Error("Cannot retrieve active tasks on worker. Skipping.", err)
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

	return workersByWork[minActiveTasks], nil
}

func (strategy *LimitActiveTasksPlacementStrategyNode) ModifiesActiveTasks() bool {
	return true
}

func (strategy *LimitActiveTasksPlacementStrategyNode) StrategyName() string {
	return strategy.GivenName
}

type LimitActiveContainersPlacementStrategyNode struct {
	GivenName     string
	maxContainers int
}

func newLimitActiveContainersPlacementStrategy(name string, maxContainers int) ContainerPlacementStrategyChainNode {
	return &LimitActiveContainersPlacementStrategyNode{
		GivenName:     name,
		maxContainers: maxContainers,
	}
}

func (strategy *LimitActiveContainersPlacementStrategyNode) Choose(logger lager.Logger, workers []Worker, spec ContainerSpec) ([]Worker, error) {
	candidates := []Worker{}

	for _, w := range workers {
		if strategy.maxContainers == 0 || w.ActiveContainers() <= strategy.maxContainers {
			candidates = append(candidates, w)
		}
	}

	return candidates, nil
}

func (strategy *LimitActiveContainersPlacementStrategyNode) ModifiesActiveTasks() bool {
	return false
}

func (strategy *LimitActiveContainersPlacementStrategyNode) StrategyName() string {
	return strategy.GivenName
}

type LimitActiveVolumesPlacementStrategyNode struct {
	GivenName  string
	maxVolumes int
}

func newLimitActiveVolumesPlacementStrategy(name string, maxVolumes int) ContainerPlacementStrategyChainNode {
	return &LimitActiveVolumesPlacementStrategyNode{
		GivenName:  name,
		maxVolumes: maxVolumes,
	}
}

func (strategy *LimitActiveVolumesPlacementStrategyNode) Choose(logger lager.Logger, workers []Worker, spec ContainerSpec) ([]Worker, error) {
	candidates := []Worker{}

	for _, w := range workers {
		if strategy.maxVolumes == 0 || w.ActiveVolumes() <= strategy.maxVolumes {
			candidates = append(candidates, w)
		}
	}

	return candidates, nil
}

func (strategy *LimitActiveVolumesPlacementStrategyNode) ModifiesActiveTasks() bool {
	return false
}

func (strategy *LimitActiveVolumesPlacementStrategyNode) StrategyName() string {
	return strategy.GivenName
}
