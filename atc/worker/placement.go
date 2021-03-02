package worker

import (
	"errors"
	"fmt"
	"math/rand"
	"strings"
	"time"

	"code.cloudfoundry.org/lager"
)

type ContainerPlacementStrategyOptions struct {
	ContainerPlacementStrategy   []string `long:"container-placement-strategy" default:"volume-locality" choice:"volume-locality" choice:"random" choice:"fewest-build-containers" choice:"limit-active-tasks" choice:"limit-active-containers" choice:"limit-active-volumes" description:"Method by which a worker is selected during container placement. If multiple methods are specified, they will be applied in order. Random strategy should only be used alone."`
	MaxActiveContainersPerWorker int      `long:"max-active-containers-per-worker" default:"0" description:"Maximum allowed number of active containers per worker. Has effect only when used with limit-active-containers placement strategy. 0 means no limit."`
	MaxActiveVolumesPerWorker    int      `long:"max-active-volumes-per-worker" default:"0" description:"Maximum allowed number of active volumes per worker. Has effect only when used with limit-active-volumes placement strategy. 0 means no limit."`
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
}

type ContainerPlacementStrategyChainNode interface {
	Choose(lager.Logger, []Worker, ContainerSpec) ([]Worker, error)
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

func (strategy *FewestBuildContainersPlacementStrategyNode) StrategyName() string {
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
	choose := []Worker{}

	for _, w := range workers {
		if strategy.maxContainers == 0 || w.ActiveContainers() <= strategy.maxContainers {
			choose = append(choose, w)
		}
	}

	return choose, nil
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
	choose := []Worker{}

	for _, w := range workers {
		if strategy.maxVolumes == 0 || w.ActiveVolumes() <= strategy.maxVolumes {
			choose = append(choose, w)
		}
	}

	return choose, nil
}

func (strategy *LimitActiveVolumesPlacementStrategyNode) StrategyName() string {
	return strategy.GivenName
}
