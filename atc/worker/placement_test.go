package worker_test

import (
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/runtime"
	"github.com/concourse/concourse/atc/worker"
	grt "github.com/concourse/concourse/atc/worker/gardenruntime/gardenruntimetest"
	"github.com/concourse/concourse/atc/worker/workertest"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/types"
)

var _ = Describe("Container Placement Strategies", func() {
	Describe("Volume Locality", func() {
		volumeLocalityStrategy := func() worker.PlacementStrategy {
			strategy, err := worker.NewPlacementStrategy(worker.PlacementOptions{
				Strategies: []string{"volume-locality"},
			})
			Expect(err).ToNot(HaveOccurred())
			return strategy
		}

		Test("sorts the workers by number of local inputs", func() {
			scenario := Setup(
				workertest.WithBasicJob(),
				workertest.WithWorkers(
					grt.NewWorker("worker1").
						WithVolumesCreatedInDBAndBaggageclaim(
							grt.NewVolume("input1"),
							grt.NewVolume("input3"),
						),
					grt.NewWorker("worker2").
						WithVolumesCreatedInDBAndBaggageclaim(
							grt.NewVolume("input2"),
						),
					grt.NewWorker("worker3"),
				),
			)

			workers, err := volumeLocalityStrategy().Order(logger, scenario.Pool, scenario.DB.Workers, runtime.ContainerSpec{
				TeamID:   scenario.TeamID,
				JobID:    scenario.JobID,
				StepName: scenario.StepName,

				Inputs: []runtime.Input{
					{
						VolumeHandle:    "input1",
						DestinationPath: "/input1",
					},
					{
						VolumeHandle:    "input2",
						DestinationPath: "/input2",
					},
					{
						VolumeHandle:    "input3",
						DestinationPath: "/input3",
					},
				},
			})
			Expect(err).ToNot(HaveOccurred())
			Expect(workerNames(workers)).To(Equal([]string{"worker1", "worker2", "worker3"}))
		})

		Test("includes all workers in the case of a tie", func() {
			scenario := Setup(
				workertest.WithBasicJob(),
				workertest.WithWorkers(
					grt.NewWorker("worker1").
						WithVolumesCreatedInDBAndBaggageclaim(
							grt.NewVolume("input1"),
						),
					grt.NewWorker("worker2").
						WithVolumesCreatedInDBAndBaggageclaim(
							grt.NewVolume("input2"),
						),
					grt.NewWorker("worker3"),
				),
			)

			workers, err := volumeLocalityStrategy().Order(logger, scenario.Pool, scenario.DB.Workers, runtime.ContainerSpec{
				TeamID:   scenario.TeamID,
				JobID:    scenario.JobID,
				StepName: scenario.StepName,

				Inputs: []runtime.Input{
					{
						VolumeHandle:    "input1",
						DestinationPath: "/input1",
					},
					{
						VolumeHandle:    "input2",
						DestinationPath: "/input2",
					},
				},
			})
			Expect(err).ToNot(HaveOccurred())
			Expect(workerNames(workers)).To(BeOneOf(
				[]string{"worker1", "worker2", "worker3"},
				[]string{"worker2", "worker1", "worker3"},
			))
		})

		Test("considers resource caches", func() {
			scenario := Setup(
				workertest.WithBasicJob(),
				workertest.WithWorkers(
					grt.NewWorker("worker1").
						WithDBContainersInState(grt.Creating, "container1").
						WithVolumesCreatedInDBAndBaggageclaim(
							grt.NewVolume("input1"),
							grt.NewVolume("cache-input2"),
						),
					grt.NewWorker("worker2").
						WithDBContainersInState(grt.Creating, "container2").
						WithVolumesCreatedInDBAndBaggageclaim(
							grt.NewVolume("input2"),
						),
				),
			)
			resourceCache1 := scenario.FindOrCreateResourceCache("worker1", "container1")
			err := scenario.WorkerVolume("worker1", "cache-input2").InitializeResourceCache(logger, resourceCache1)
			Expect(err).ToNot(HaveOccurred())

			resourceCache2 := scenario.FindOrCreateResourceCache("worker2", "container2")
			err = scenario.WorkerVolume("worker2", "input2").InitializeResourceCache(logger, resourceCache2)
			Expect(err).ToNot(HaveOccurred())

			workers, err := volumeLocalityStrategy().Order(logger, scenario.Pool, scenario.DB.Workers, runtime.ContainerSpec{
				TeamID:   scenario.TeamID,
				JobID:    scenario.JobID,
				StepName: scenario.StepName,

				Inputs: []runtime.Input{
					{
						VolumeHandle:    "input1",
						DestinationPath: "/input1",
					},
					{
						VolumeHandle:    "input2",
						DestinationPath: "/input2",
					},
				},
			})
			Expect(err).ToNot(HaveOccurred())
			Expect(workerNames(workers)).To(Equal([]string{"worker1", "worker2"}))
		})

		Test("considers task caches", func() {
			scenario := Setup(
				workertest.WithBasicJob(),
				workertest.WithWorkers(
					grt.NewWorker("worker1").
						WithVolumesCreatedInDBAndBaggageclaim(
							grt.NewVolume("input1"),
							grt.NewVolume("cache1_worker1"),
							grt.NewVolume("cache2_worker1"),
						),
					grt.NewWorker("worker2").
						WithVolumesCreatedInDBAndBaggageclaim(
							grt.NewVolume("input2"),
							grt.NewVolume("cache1_worker2"),
						),
				),
			)

			err := scenario.WorkerVolume("worker1", "cache1_worker1").
				InitializeTaskCache(logger, scenario.JobID, scenario.StepName, "/cache1", false)
			Expect(err).ToNot(HaveOccurred())
			err = scenario.WorkerVolume("worker1", "cache2_worker1").
				InitializeTaskCache(logger, scenario.JobID, scenario.StepName, "/cache2", false)
			Expect(err).ToNot(HaveOccurred())
			err = scenario.WorkerVolume("worker2", "cache1_worker2").
				InitializeTaskCache(logger, scenario.JobID, scenario.StepName, "/cache1", false)
			Expect(err).ToNot(HaveOccurred())

			workers, err := volumeLocalityStrategy().Order(logger, scenario.Pool, scenario.DB.Workers, runtime.ContainerSpec{
				TeamID:   scenario.TeamID,
				JobID:    scenario.JobID,
				StepName: scenario.StepName,

				Inputs: []runtime.Input{
					{
						VolumeHandle:    "input1",
						DestinationPath: "/input1",
					},
					{
						VolumeHandle:    "input2",
						DestinationPath: "/input2",
					},
				},

				Caches: []string{"/cache1", "/cache2"},
			})
			Expect(err).ToNot(HaveOccurred())
			Expect(workerNames(workers)).To(Equal([]string{"worker1", "worker2"}))
		})

		Test("does not consider workers that have been filtered out", func() {
			scenario := Setup(
				workertest.WithBasicJob(),
				workertest.WithWorkers(
					grt.NewWorker("worker1"),
					grt.NewWorker("worker2").
						WithVolumesCreatedInDBAndBaggageclaim(
							grt.NewVolume("input1"),
						),
				),
			)

			workers, err := volumeLocalityStrategy().Order(
				logger,
				scenario.Pool,
				filterWorkers(scenario.DB.Workers, "worker1"),
				runtime.ContainerSpec{
					TeamID:   scenario.TeamID,
					JobID:    scenario.JobID,
					StepName: scenario.StepName,

					Inputs: []runtime.Input{
						{
							VolumeHandle:    "input1",
							DestinationPath: "/input1",
						},
					},
				},
			)
			Expect(err).ToNot(HaveOccurred())
			Expect(workerNames(workers)).To(Equal([]string{"worker1"}))
		})
	})

	Describe("Fewest Build Containers", func() {
		fewestBuildContainersStrategy := func() worker.PlacementStrategy {
			strategy, err := worker.NewPlacementStrategy(worker.PlacementOptions{
				Strategies: []string{"fewest-build-containers"},
			})
			Expect(err).ToNot(HaveOccurred())
			return strategy
		}

		Test("returns workers with the fewest active containers", func() {
			scenario := Setup(
				workertest.WithBasicJob(),
				workertest.WithWorkers(
					grt.NewWorker("worker1").
						WithContainersCreatedInDBAndGarden(
							grt.NewContainer("c1"),
						),
					grt.NewWorker("worker2").
						WithContainersCreatedInDBAndGarden(
							grt.NewContainer("c2"),
							grt.NewContainer("c3"),
						),
					grt.NewWorker("worker3").
						WithContainersCreatedInDBAndGarden(
							grt.NewContainer("c4"),
						),
				),
			)

			workers, err := fewestBuildContainersStrategy().Order(logger, scenario.Pool, scenario.DB.Workers, runtime.ContainerSpec{
				TeamID:   scenario.TeamID,
				JobID:    scenario.JobID,
				StepName: scenario.StepName,
			})
			Expect(err).ToNot(HaveOccurred())
			Expect(workerNames(workers)).To(BeOneOf(
				[]string{"worker1", "worker3", "worker2"},
				[]string{"worker3", "worker1", "worker2"},
			))
		})
	})

	Describe("Limit Active Tasks", func() {
		limitActiveTasksStrategy := func(max int) worker.PlacementStrategy {
			strategy, err := worker.NewPlacementStrategy(worker.PlacementOptions{
				Strategies:              []string{"limit-active-tasks"},
				MaxActiveTasksPerWorker: max,
			})
			Expect(err).ToNot(HaveOccurred())
			return strategy
		}

		Test("returns workers with the fewest active tasks", func() {
			scenario := Setup(
				workertest.WithBasicJob(),
				workertest.WithWorkers(
					grt.NewWorker("worker1").
						WithActiveTasks(1),
					grt.NewWorker("worker2").
						WithActiveTasks(2),
					grt.NewWorker("worker3").
						WithActiveTasks(1),
				),
			)

			workers, err := limitActiveTasksStrategy(0).Order(logger, scenario.Pool, scenario.DB.Workers, runtime.ContainerSpec{
				TeamID:   scenario.TeamID,
				JobID:    scenario.JobID,
				StepName: scenario.StepName,

				Type: db.ContainerTypeTask,
			})
			Expect(err).ToNot(HaveOccurred())
			Expect(workerNames(workers)).To(BeOneOf(
				[]string{"worker1", "worker3", "worker2"},
				[]string{"worker3", "worker1", "worker2"},
			))
		})

		Test("allows setting a limit on the number of active tasks", func() {
			scenario := Setup(
				workertest.WithBasicJob(),
				workertest.WithWorkers(
					grt.NewWorker("worker1").
						WithActiveTasks(10),
					grt.NewWorker("worker2").
						WithActiveTasks(20),
					grt.NewWorker("worker3").
						WithActiveTasks(10),
				),
			)

			strategy := limitActiveTasksStrategy(10)
			spec := runtime.ContainerSpec{
				TeamID:   scenario.TeamID,
				JobID:    scenario.JobID,
				StepName: scenario.StepName,

				Type: db.ContainerTypeTask,
			}

			workers, err := strategy.Order(logger, scenario.Pool, scenario.DB.Workers, spec)
			Expect(err).ToNot(HaveOccurred())

			err = strategy.Pick(logger, workers[0], spec)
			Expect(err).To(MatchError(worker.ErrTooManyActiveTasks))

			By("validating the limit only applies to task containers", func() {
				spec.Type = db.ContainerTypeCheck

				workers, err := strategy.Order(logger, scenario.Pool, scenario.DB.Workers, spec)
				Expect(err).ToNot(HaveOccurred())

				err = strategy.Pick(logger, workers[0], spec)
				Expect(err).ToNot(HaveOccurred())
			})
		})
	})

	Describe("Limit Active Containers", func() {
		limitActiveContainersStrategy := func(max int) worker.PlacementStrategy {
			strategy, err := worker.NewPlacementStrategy(worker.PlacementOptions{
				Strategies:                   []string{"limit-active-containers"},
				MaxActiveContainersPerWorker: max,
			})
			Expect(err).ToNot(HaveOccurred())
			return strategy
		}

		Test("disallows workers with too many active containers", func() {
			scenario := Setup(
				workertest.WithBasicJob(),
				workertest.WithWorkers(
					grt.NewWorker("worker1").
						WithContainersCreatedInDBAndGarden(
							grt.NewContainer("c1"),
						),
					grt.NewWorker("worker2").
						WithContainersCreatedInDBAndGarden(
							grt.NewContainer("c2"),
							grt.NewContainer("c3"),
						),
					grt.NewWorker("worker3").
						WithContainersCreatedInDBAndGarden(
							grt.NewContainer("c4"),
						),
				),
			)

			strategy := limitActiveContainersStrategy(2)
			spec := runtime.ContainerSpec{
				TeamID:   scenario.TeamID,
				JobID:    scenario.JobID,
				StepName: scenario.StepName,
			}

			workers, err := strategy.Order(logger, scenario.Pool, scenario.DB.Workers, spec)
			Expect(err).ToNot(HaveOccurred())
			Expect(workerNames(workers)).To(BeOneOf(
				[]string{"worker1", "worker3", "worker2"},
				[]string{"worker3", "worker1", "worker2"},
			))

			err = strategy.Pick(logger, workers[0], spec)
			Expect(err).ToNot(HaveOccurred())

			err = strategy.Pick(logger, workers[2], spec)
			Expect(err).To(MatchError(worker.ErrTooManyContainers))
		})

		Test("noop if limit is unset", func() {
			scenario := Setup(
				workertest.WithBasicJob(),
				workertest.WithWorkers(
					grt.NewWorker("worker1").
						WithContainersCreatedInDBAndGarden(
							grt.NewContainer("c1"),
						),
					grt.NewWorker("worker2").
						WithContainersCreatedInDBAndGarden(
							grt.NewContainer("c2"),
							grt.NewContainer("c3"),
						),
					grt.NewWorker("worker3").
						WithContainersCreatedInDBAndGarden(
							grt.NewContainer("c4"),
						),
				),
			)

			strategy := limitActiveContainersStrategy(0)
			spec := runtime.ContainerSpec{
				TeamID:   scenario.TeamID,
				JobID:    scenario.JobID,
				StepName: scenario.StepName,
			}

			workers, err := strategy.Order(logger, scenario.Pool, scenario.DB.Workers, spec)
			Expect(err).ToNot(HaveOccurred())

			for _, worker := range workers {
				err := strategy.Pick(logger, worker, spec)
				Expect(err).ToNot(HaveOccurred())
			}
		})
	})

	Describe("Limit Active Volumes", func() {
		limitActiveVolumesStrategy := func(max int) worker.PlacementStrategy {
			strategy, err := worker.NewPlacementStrategy(worker.PlacementOptions{
				Strategies:                []string{"limit-active-volumes"},
				MaxActiveVolumesPerWorker: max,
			})
			Expect(err).ToNot(HaveOccurred())
			return strategy
		}

		Test("removes workers with too many active volumes", func() {
			scenario := Setup(
				workertest.WithBasicJob(),
				workertest.WithWorkers(
					grt.NewWorker("worker1").
						WithVolumesCreatedInDBAndBaggageclaim(
							grt.NewVolume("v1"),
						),
					grt.NewWorker("worker2").
						WithVolumesCreatedInDBAndBaggageclaim(
							grt.NewVolume("v2"),
							grt.NewVolume("v3"),
						),
					grt.NewWorker("worker3").
						WithVolumesCreatedInDBAndBaggageclaim(
							grt.NewVolume("v4"),
						),
				),
			)

			strategy := limitActiveVolumesStrategy(2)
			spec := runtime.ContainerSpec{
				TeamID:   scenario.TeamID,
				JobID:    scenario.JobID,
				StepName: scenario.StepName,
			}

			workers, err := strategy.Order(logger, scenario.Pool, scenario.DB.Workers, spec)
			Expect(err).ToNot(HaveOccurred())
			Expect(workerNames(workers)).To(BeOneOf(
				[]string{"worker1", "worker3", "worker2"},
				[]string{"worker3", "worker1", "worker2"},
			))

			err = strategy.Pick(logger, workers[0], spec)
			Expect(err).ToNot(HaveOccurred())

			err = strategy.Pick(logger, workers[2], spec)
			Expect(err).To(MatchError(worker.ErrTooManyVolumes))
		})

		Test("noop if limit is unset", func() {
			scenario := Setup(
				workertest.WithBasicJob(),
				workertest.WithWorkers(
					grt.NewWorker("worker1").
						WithVolumesCreatedInDBAndBaggageclaim(
							grt.NewVolume("v1"),
						),
					grt.NewWorker("worker2").
						WithVolumesCreatedInDBAndBaggageclaim(
							grt.NewVolume("v2"),
							grt.NewVolume("v3"),
						),
					grt.NewWorker("worker3").
						WithVolumesCreatedInDBAndBaggageclaim(
							grt.NewVolume("v4"),
						),
				),
			)

			strategy := limitActiveVolumesStrategy(0)
			spec := runtime.ContainerSpec{
				TeamID:   scenario.TeamID,
				JobID:    scenario.JobID,
				StepName: scenario.StepName,
			}

			workers, err := strategy.Order(logger, scenario.Pool, scenario.DB.Workers, spec)
			Expect(err).ToNot(HaveOccurred())

			for _, worker := range workers {
				err := strategy.Pick(logger, worker, spec)
				Expect(err).ToNot(HaveOccurred())
			}
		})
	})
})

func BeOneOf(vals ...interface{}) types.GomegaMatcher {
	matchers := make([]types.GomegaMatcher, len(vals))
	for i, v := range vals {
		matchers[i] = Equal(v)
	}
	return SatisfyAny(matchers...)
}

func workerNames(workers []db.Worker) []string {
	names := make([]string, len(workers))
	for i, worker := range workers {
		names[i] = worker.Name()
	}
	return names
}

func filterWorkers(allWorkers []db.Worker, namesToKeep ...string) []db.Worker {
	keep := func(name string) bool {
		for _, otherName := range namesToKeep {
			if name == otherName {
				return true
			}
		}
		return false
	}

	var workers []db.Worker
	for _, worker := range allWorkers {
		if keep(worker.Name()) {
			workers = append(workers, worker)
		}
	}
	return workers
}
