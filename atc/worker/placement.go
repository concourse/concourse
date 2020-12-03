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
	ContainerPlacementStrategy   []string `long:"container-placement-strategy" default:"volume-locality" choice:"volume-locality" choice:"random" choice:"fewest-build-containers" choice:"limit-active-tasks" choice:"limit-active-containers" choice:"limit-active-volumes" description:"Method by which a worker is selected during container placement. If multiple methods are specified, they will be applied in order. Random strategy should only be used alone."`
	MaxActiveTasksPerWorker      int      `long:"max-active-tasks-per-worker" default:"0" description:"Maximum allowed number of active build tasks per worker. Has effect only when used with limit-active-tasks placement strategy. 0 means no limit."`
	MaxActiveContainersPerWorker int      `long:"max-active-containers-per-worker" default:"0" description:"Maximum allowed number of active containers per worker. Has effect only when used with limit-active-containers placement strategy. 0 means no limit."`
	MaxActiveVolumesPerWorker    int      `long:"max-active-volumes-per-worker" default:"0" description:"Maximum allowed number of active volumes per worker. Has effect only when used with limit-active-volumes placement strategy. 0 means no limit."`
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
			cps.nodes = append(cps.nodes, newFewestBuildContainersPlacementStrategy())
		case "limit-active-tasks":
			if opts.MaxActiveTasksPerWorker < 0 {
				return nil, errors.New("max-active-tasks-per-worker must be greater or equal than 0")
			}
			cps.nodes = append(cps.nodes, newLimitActiveTasksPlacementStrategy(opts.MaxActiveTasksPerWorker))
		case "limit-active-containers":
			if opts.MaxActiveContainersPerWorker < 0 {
				return nil, errors.New("max-active-containers-per-worker must be greater or equal than 0")
			}
			cps.nodes = append(cps.nodes, newLimitActiveContainersPlacementStrategy(opts.MaxActiveContainersPerWorker))
		case "limit-active-volumes":
			if opts.MaxActiveVolumesPerWorker < 0 {
				return nil, errors.New("max-active-volumes-per-worker must be greater or equal than 0")
			}
			cps.nodes = append(cps.nodes, newLimitActiveVolumesPlacementStrategy(opts.MaxActiveVolumesPerWorker))
		case "volume-locality":
			cps.nodes = append(cps.nodes, newVolumeLocalityPlacementStrategyNode())
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
			return nil, nil
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

type VolumeLocalityPlacementStrategyNode struct{}

func newVolumeLocalityPlacementStrategyNode() ContainerPlacementStrategyChainNode {
	return &VolumeLocalityPlacementStrategyNode{}
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

type FewestBuildContainersPlacementStrategyNode struct{}

func newFewestBuildContainersPlacementStrategy() ContainerPlacementStrategyChainNode {
	return &FewestBuildContainersPlacementStrategyNode{}
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

type LimitActiveTasksPlacementStrategyNode struct {
	maxTasks int
}

func newLimitActiveTasksPlacementStrategy(maxTasks int) ContainerPlacementStrategyChainNode {
	return &LimitActiveTasksPlacementStrategyNode{
		maxTasks: maxTasks,
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

type LimitActiveContainersPlacementStrategyNode struct {
	maxContainers int
}

func newLimitActiveContainersPlacementStrategy(maxContainers int) ContainerPlacementStrategyChainNode {
	return &LimitActiveContainersPlacementStrategyNode{
		maxContainers: maxContainers,
	}
}

func (strategy *LimitActiveContainersPlacementStrategyNode) Choose(logger lager.Logger, workers []Worker, spec ContainerSpec) ([]Worker, error) {
	candidates := []Worker{}

	for _, w := range workers {
		if w.ActiveContainers() <= strategy.maxContainers {
			candidates = append(candidates, w)
		}
	}

	return candidates, nil
}

func (strategy *LimitActiveContainersPlacementStrategyNode) ModifiesActiveTasks() bool {
	return false
}

type LimitActiveVolumesPlacementStrategyNode struct {
	maxVolumes int
}

func newLimitActiveVolumesPlacementStrategy(maxVolumes int) ContainerPlacementStrategyChainNode {
	return &LimitActiveVolumesPlacementStrategyNode{
		maxVolumes: maxVolumes,
	}
}

func (strategy *LimitActiveVolumesPlacementStrategyNode) Choose(logger lager.Logger, workers []Worker, spec ContainerSpec) ([]Worker, error) {
	candidates := []Worker{}

	for _, w := range workers {
		if w.ActiveVolumes() <= strategy.maxVolumes {
			candidates = append(candidates, w)
		}
	}

	return candidates, nil
}

func (strategy *LimitActiveVolumesPlacementStrategyNode) ModifiesActiveTasks() bool {
	return false
}
