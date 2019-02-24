package worker

import (
	"math/rand"
	"time"

	"github.com/concourse/concourse/atc/db"

	"code.cloudfoundry.org/lager"
)

type ContainerPlacementStrategy interface {
	//TODO: Don't pass around container metadata since it's not guaranteed to be deterministic.
	// Change this after check containers stop being reused
	Choose(lager.Logger, []Worker, ContainerSpec, db.ContainerMetadata) (Worker, error)
}

type VolumeLocalityPlacementStrategy struct {
	rand *rand.Rand
}

func NewVolumeLocalityPlacementStrategy() ContainerPlacementStrategy {
	return &VolumeLocalityPlacementStrategy{
		rand: rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

func (strategy *VolumeLocalityPlacementStrategy) Choose(logger lager.Logger, workers []Worker, spec ContainerSpec, metadata db.ContainerMetadata) (Worker, error) {
	workersByCount := map[int][]Worker{}
	var highestCount int
	for _, w := range workers {
		candidateInputCount := 0

		for _, inputSource := range spec.Inputs {
			_, found, err := inputSource.Source().VolumeOn(logger, w)
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

type FewestBuildContainersPlacementStrategy struct {
	rand *rand.Rand
}

func NewFewestBuildContainersPlacementStrategy() ContainerPlacementStrategy {
	return &FewestBuildContainersPlacementStrategy{
		rand: rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

func (strategy *FewestBuildContainersPlacementStrategy) Choose(logger lager.Logger, workers []Worker, spec ContainerSpec, metadata db.ContainerMetadata) (Worker, error) {
	workersByWork := map[int][]Worker{}
	var minWork int

	// TODO: we want to remove this in the future when we don't reuse check containers
	if metadata.Type == db.ContainerTypeCheck {
		return workers[strategy.rand.Intn(len(workers))], nil
	}

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

type RandomPlacementStrategy struct {
	rand *rand.Rand
}

func NewRandomPlacementStrategy() ContainerPlacementStrategy {
	return &RandomPlacementStrategy{
		rand: rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

func (strategy *RandomPlacementStrategy) Choose(logger lager.Logger, workers []Worker, spec ContainerSpec, metadata db.ContainerMetadata) (Worker, error) {
	return workers[strategy.rand.Intn(len(workers))], nil
}
