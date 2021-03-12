package worker2_test

import (
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/runtime"
	"github.com/concourse/concourse/atc/worker2"
	grt "github.com/concourse/concourse/atc/worker2/gardenruntime/gardenruntimetest"
	"github.com/concourse/concourse/atc/worker2/workertest"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Pool", func() {
	Describe("FindOrSelectWorker", func() {
		Test("find a worker with an existing container", func() {
			scenario := Setup(
				workertest.WithWorkers(
					grt.NewWorker("worker1"),
					grt.NewWorker("worker2").
						WithDBContainersInState(grt.Creating, "my-container"),
					grt.NewWorker("worker3"),
				),
			)

			worker, err := scenario.Pool.FindOrSelectWorker(
				logger,
				db.NewFixedHandleContainerOwner("my-container"),
				runtime.ContainerSpec{},
				worker2.Spec{},
				nil,
			)
			Expect(err).ToNot(HaveOccurred())

			Expect(worker.Name()).To(Equal("worker2"))
		})

		Test("selects a worker when container owner has no worker", func() {
			scenario := Setup(
				workertest.WithWorkers(
					grt.NewWorker("worker1"),
					grt.NewWorker("worker2"),
					grt.NewWorker("worker3"),
				),
			)

			worker, err := scenario.Pool.FindOrSelectWorker(
				logger,
				db.NewFixedHandleContainerOwner("no-worker-for-this-container-yet"),
				runtime.ContainerSpec{},
				worker2.Spec{},
				nil,
			)
			Expect(err).ToNot(HaveOccurred())

			Expect(worker.Name()).To(BeOneOf("worker1", "worker2", "worker3"))
		})

		Test("follows the strategy for selecting a worker", func() {
			scenario := Setup(
				workertest.WithWorkers(
					grt.NewWorker("worker1").
						WithContainersCreatedInDBAndGarden(
							grt.NewContainer("c1"),
							grt.NewContainer("c2"),
						),
					grt.NewWorker("worker2").
						WithContainersCreatedInDBAndGarden(
							grt.NewContainer("c3"),
						),
					grt.NewWorker("worker3"),
				),
			)

			strategy, err := worker2.NewPlacementStrategy(worker2.PlacementOptions{
				Strategies: []string{"fewest-build-containers"},
			})
			Expect(err).ToNot(HaveOccurred())

			worker, err := scenario.Pool.FindOrSelectWorker(
				logger,
				db.NewFixedHandleContainerOwner("no-worker-for-this-container-yet"),
				runtime.ContainerSpec{},
				worker2.Spec{},
				strategy,
			)
			Expect(err).ToNot(HaveOccurred())

			Expect(worker.Name()).To(Equal("worker3"))
		})

		Test("selects a new worker when owning worker is incompatible", func() {
			scenario := Setup(
				workertest.WithWorkers(
					grt.NewWorker("worker1"),
					grt.NewWorker("worker2").
						WithContainersCreatedInDBAndGarden(
							grt.NewContainer("my-container"),
						).
						WithVersion("0.1"),
				),
			)

			worker, err := scenario.Pool.FindOrSelectWorker(
				logger,
				db.NewFixedHandleContainerOwner("my-container"),
				runtime.ContainerSpec{},
				worker2.Spec{},
				nil,
			)
			Expect(err).ToNot(HaveOccurred())

			Expect(worker.Name()).To(Equal("worker1"))
		})

		Test("filters out incompatible workers by resource type", func() {
			scenario := Setup(
				workertest.WithWorkers(
					grt.NewWorker("worker1"),
					grt.NewWorker("worker2"),
				),
			)

			_, err := scenario.Pool.FindOrSelectWorker(
				logger,
				db.NewFixedHandleContainerOwner("my-container"),
				runtime.ContainerSpec{},
				worker2.Spec{
					ResourceType: "busted-resource-type",
				},
				nil,
			)
			Expect(err).To(MatchError(ContainSubstring("no workers satisfying")))
		})

		Test("filters out incompatible workers by platform", func() {
			scenario := Setup(
				workertest.WithWorkers(
					grt.NewWorker("worker1"),
					grt.NewWorker("worker2"),
				),
			)

			_, err := scenario.Pool.FindOrSelectWorker(
				logger,
				db.NewFixedHandleContainerOwner("my-container"),
				runtime.ContainerSpec{},
				worker2.Spec{
					Platform: "random-platform",
				},
				nil,
			)
			Expect(err).To(MatchError(ContainSubstring("no workers satisfying")))
		})

		Test("filters out incompatible workers by tags", func() {
			scenario := Setup(
				workertest.WithWorkers(
					grt.NewWorker("worker1").WithTags("A", "C"),
					grt.NewWorker("worker2").WithTags("B", "C"),
					grt.NewWorker("worker3"),
				),
			)

			_, err := scenario.Pool.FindOrSelectWorker(
				logger,
				db.NewFixedHandleContainerOwner("my-container"),
				runtime.ContainerSpec{},
				worker2.Spec{
					Tags: []string{"A", "B"},
				},
				nil,
			)
			Expect(err).To(MatchError(ContainSubstring("no workers satisfying")))
		})

		Test("only considers team workers when any team worker is compatible", func() {
			scenario := Setup(
				workertest.WithTeam("team"),
				workertest.WithWorkers(
					grt.NewWorker("worker1").WithTeam("team"),
					grt.NewWorker("worker2"),
					grt.NewWorker("worker3"),
				),
			)

			worker, err := scenario.Pool.FindOrSelectWorker(
				logger,
				db.NewFixedHandleContainerOwner("my-container"),
				runtime.ContainerSpec{},
				worker2.Spec{
					TeamID: scenario.Team("team").ID(),
				},
				nil,
			)
			Expect(err).ToNot(HaveOccurred())

			Expect(worker.Name()).To(Equal("worker1"))
		})

		Test("considers general workers when all team workers are incompatible", func() {
			scenario := Setup(
				workertest.WithTeam("team"),
				workertest.WithWorkers(
					grt.NewWorker("worker1").WithTeam("team").WithPlatform("dummy"),
					grt.NewWorker("worker2"),
					grt.NewWorker("worker3"),
				),
			)

			worker, err := scenario.Pool.FindOrSelectWorker(
				logger,
				db.NewFixedHandleContainerOwner("my-container"),
				runtime.ContainerSpec{},
				worker2.Spec{
					Platform: "linux",
					TeamID:   scenario.Team("team").ID(),
				},
				nil,
			)
			Expect(err).ToNot(HaveOccurred())

			Expect(worker.Name()).To(BeOneOf("worker2", "worker3"))
		})

		Test("no worker satisfies strategy", func() {
			scenario := Setup(
				workertest.WithWorkers(
					grt.NewWorker("worker1").
						WithContainersCreatedInDBAndGarden(
							grt.NewContainer("c1"),
							grt.NewContainer("c2"),
						),
					grt.NewWorker("worker2").
						WithContainersCreatedInDBAndGarden(
							grt.NewContainer("c3"),
							grt.NewContainer("c4"),
						),
				),
			)

			strategy, err := worker2.NewPlacementStrategy(worker2.PlacementOptions{
				Strategies:                   []string{"limit-active-containers"},
				MaxActiveContainersPerWorker: 1,
			})
			Expect(err).ToNot(HaveOccurred())

			_, err = scenario.Pool.FindOrSelectWorker(
				logger,
				db.NewFixedHandleContainerOwner("my-container"),
				runtime.ContainerSpec{},
				worker2.Spec{},
				strategy,
			)
			Expect(err).To(MatchError("no worker fit container placement strategy: limit-active-containers"))
		})
	})
})
