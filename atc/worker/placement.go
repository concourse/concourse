package worker

import (
	"errors"
	"fmt"
	"strings"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/concourse/atc/db"
)

type ContainerPlacementStrategyOptions struct {
	ContainerPlacementStrategy   []string `long:"container-placement-strategy" default:"volume-locality" choice:"volume-locality" choice:"random" choice:"fewest-build-containers" choice:"limit-active-tasks" choice:"limit-active-containers" choice:"limit-active-volumes" description:"Method by which a worker is selected during container placement. If multiple methods are specified, they will be applied in order. Random strategy should only be used alone."`
	MaxActiveTasksPerWorker      int      `long:"max-active-tasks-per-worker" default:"0" description:"Maximum allowed number of active build tasks per worker. Has effect only when used with limit-active-tasks placement strategy. 0 means no limit."`
	MaxActiveContainersPerWorker int      `long:"max-active-containers-per-worker" default:"0" description:"Maximum allowed number of active containers per worker. Has effect only when used with limit-active-containers placement strategy. 0 means no limit."`
	MaxActiveVolumesPerWorker    int      `long:"max-active-volumes-per-worker" default:"0" description:"Maximum allowed number of active volumes per worker. Has effect only when used with limit-active-volumes placement strategy. 0 means no limit."`
}

var (
	ErrFailedAcquirePlacementLock = errors.New("failed to acquire placement lock")
)

type NoWorkerFitContainerPlacementStrategyError struct {
	Strategy string
}

func (err NoWorkerFitContainerPlacementStrategyError) Error() string {
	return fmt.Sprintf("no worker fit container placement strategy: %s", err.Strategy)
}

type ContainerPlacementStrategy interface {
	//TODO: Don't pass around container metadata since it's not guaranteed to be deterministic.
	// Change this after check containers stop being reused
	Candidates(lager.Logger, []Worker, ContainerSpec) ([]Worker, error)
	Pick(lager.Logger, Worker, ContainerSpec) error
}

type ContainerPlacementStrategyChainNode interface {
	Candidates(lager.Logger, []Worker, ContainerSpec) ([]Worker, error)
	Pick(lager.Logger, Worker, ContainerSpec) error

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

func (strategy *containerPlacementStrategy) Candidates(logger lager.Logger, workers []Worker, spec ContainerSpec) ([]Worker, error) {
	var err error
	for _, node := range strategy.nodes {
		workers, err = node.Candidates(logger, workers, spec)
		if err != nil {
			return nil, err
		}

		if len(workers) == 0 {
			return nil, NoWorkerFitContainerPlacementStrategyError{Strategy: node.StrategyName()}
		}
	}

	return workers, nil
}

func (strategy *containerPlacementStrategy) Pick(logger lager.Logger, worker Worker, spec ContainerSpec) error {
	for _, node := range strategy.nodes {
		err := node.Pick(logger, worker, spec)

		if err != nil {
			return err
		}
	}

	return nil
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

func (strategy *VolumeLocalityPlacementStrategyNode) Candidates(logger lager.Logger, workers []Worker, spec ContainerSpec) ([]Worker, error) {
	candidates := map[int][]Worker{}
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

		candidates[candidateInputCount] = append(candidates[candidateInputCount], w)

		if candidateInputCount >= highestCount {
			highestCount = candidateInputCount
		}
	}

	if len(candidates) == 0 {
		return []Worker{}, nil
	}

	return candidates[highestCount], nil
}

func (strategy *VolumeLocalityPlacementStrategyNode) Pick(logger lager.Logger, worker Worker, spec ContainerSpec) error {
	return nil
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

func (strategy *FewestBuildContainersPlacementStrategyNode) Candidates(logger lager.Logger, workers []Worker, spec ContainerSpec) ([]Worker, error) {
	candidates := map[int][]Worker{}
	var minWork int

	for i, w := range workers {
		work := w.BuildContainers()
		candidates[work] = append(candidates[work], w)

		if i == 0 || work < minWork {
			minWork = work
		}
	}

	if len(candidates) == 0 {
		return []Worker{}, nil
	}

	return candidates[minWork], nil
}

func (strategy *FewestBuildContainersPlacementStrategyNode) Pick(logger lager.Logger, worker Worker, spec ContainerSpec) error {
	return nil
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

func (strategy *LimitActiveTasksPlacementStrategyNode) Candidates(logger lager.Logger, workers []Worker, spec ContainerSpec) ([]Worker, error) {
	candidates := []Worker{}

	for _, w := range workers {
		activeTasks, err := w.ActiveTasks()
		if err != nil {
			logger.Error("Cannot retrieve active tasks on worker. Skipping.", err)
			continue
		}

		// If maxTasks == 0 or the step is not a task, ignore the number of active tasks and distribute the work evenly
		if spec.Type == db.ContainerTypeTask && (strategy.maxTasks == 0 || activeTasks <= strategy.maxTasks) {
			candidates = append(candidates, w)
		}
	}

	return candidates, nil
}

func (strategy *LimitActiveTasksPlacementStrategyNode) Pick(logger lager.Logger, worker Worker, spec ContainerSpec) error {
	activeTasks, err := worker.ActiveTasks()
	if err != nil {
		return err
	}

	if spec.Type == db.ContainerTypeTask && (strategy.maxTasks == 0 || activeTasks <= strategy.maxTasks) {
		return errors.New("active tasks on worker over configured limit")
	}

	return nil
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

func (strategy *LimitActiveContainersPlacementStrategyNode) Candidates(logger lager.Logger, workers []Worker, spec ContainerSpec) ([]Worker, error) {
	candidates := []Worker{}

	for _, w := range workers {
		if strategy.maxContainers == 0 || w.ActiveContainers() <= strategy.maxContainers {
			candidates = append(candidates, w)
		}
	}

	return candidates, nil
}

func (strategy *LimitActiveContainersPlacementStrategyNode) Pick(logger lager.Logger, worker Worker, spec ContainerSpec) error {
	return nil
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

func (strategy *LimitActiveVolumesPlacementStrategyNode) Candidates(logger lager.Logger, workers []Worker, spec ContainerSpec) ([]Worker, error) {
	candidates := []Worker{}

	for _, w := range workers {
		if strategy.maxVolumes == 0 || w.ActiveVolumes() <= strategy.maxVolumes {
			candidates = append(candidates, w)
		}
	}

	return candidates, nil
}

func (strategy *LimitActiveVolumesPlacementStrategyNode) Pick(logger lager.Logger, worker Worker, spec ContainerSpec) error {
	return nil
}

func (strategy *LimitActiveVolumesPlacementStrategyNode) StrategyName() string {
	return strategy.GivenName
}
