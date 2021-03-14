package worker_test

import (
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/runtime"
	"github.com/concourse/concourse/atc/worker"
	grt "github.com/concourse/concourse/atc/worker/gardenruntime/gardenruntimetest"
	"github.com/concourse/concourse/atc/worker/workerfakes"
	"github.com/concourse/concourse/atc/worker/workertest"
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
				worker.Spec{},
				nil,
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
				worker.Spec{},
				nil,
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

			strategy, err := worker.NewPlacementStrategy(worker.PlacementOptions{
				Strategies: []string{"fewest-build-containers"},
			})
			Expect(err).ToNot(HaveOccurred())

			worker, err := scenario.Pool.FindOrSelectWorker(
				logger,
				db.NewFixedHandleContainerOwner("no-worker-for-this-container-yet"),
				runtime.ContainerSpec{},
				worker.Spec{},
				strategy,
				nil,
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
				worker.Spec{},
				nil,
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
				worker.Spec{
					ResourceType: "busted-resource-type",
				},
				nil,
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
				worker.Spec{
					Platform: "random-platform",
				},
				nil,
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
				worker.Spec{
					Tags: []string{"A", "B"},
				},
				nil,
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
				worker.Spec{
					TeamID: scenario.Team("team").ID(),
				},
				nil,
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
				worker.Spec{
					Platform: "linux",
					TeamID:   scenario.Team("team").ID(),
				},
				nil,
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

			strategy, err := worker.NewPlacementStrategy(worker.PlacementOptions{
				Strategies:                   []string{"limit-active-containers"},
				MaxActiveContainersPerWorker: 1,
			})
			Expect(err).ToNot(HaveOccurred())

			_, err = scenario.Pool.FindOrSelectWorker(
				logger,
				db.NewFixedHandleContainerOwner("my-container"),
				runtime.ContainerSpec{},
				worker.Spec{},
				strategy,
				nil,
			)
			Expect(err).To(MatchError("no worker fit container placement strategy: limit-active-containers"))
		})
	})

	Describe("FindWorkersForResourceCache", func() {
		var (
			workerSpec WorkerSpec

			chosenWorkers []Worker
			chooseErr     error

			incompatibleWorker *workerfakes.FakeWorker
			compatibleWorker   *workerfakes.FakeWorker
		)

		BeforeEach(func() {
			workerSpec = WorkerSpec{
				ResourceType: "some-type",
				TeamID:       4567,
				Tags:         atc.Tags{"some-tag"},
			}

			incompatibleWorker = new(workerfakes.FakeWorker)
			incompatibleWorker.SatisfiesReturns(false)

			compatibleWorker = new(workerfakes.FakeWorker)
			compatibleWorker.SatisfiesReturns(true)
		})

		JustBeforeEach(func() {
			chosenWorkers, chooseErr = pool.FindWorkersForResourceCache(
				logger,
				4567,
				1234,
				workerSpec,
			)
		})

		Context("when workers are found with the resource cache", func() {
			var (
				workerA *workerfakes.FakeWorker
				workerB *workerfakes.FakeWorker
				workerC *workerfakes.FakeWorker
			)

			BeforeEach(func() {
				workerA = new(workerfakes.FakeWorker)
				workerA.NameReturns("workerA")
				workerB = new(workerfakes.FakeWorker)
				workerB.NameReturns("workerB")
				workerC = new(workerfakes.FakeWorker)
				workerC.NameReturns("workerC")

				fakeProvider.FindWorkersForResourceCacheReturns([]Worker{workerA, workerB, workerC}, nil)
			})

			Context("when one of the workers satisfy the spec", func() {
				BeforeEach(func() {
					workerA.SatisfiesReturns(true)
					workerB.SatisfiesReturns(false)
					workerC.SatisfiesReturns(false)
				})

				It("succeeds and returns the compatible worker with the resource cache", func() {
					Expect(chooseErr).NotTo(HaveOccurred())
					Expect(len(chosenWorkers)).To(Equal(1))
					Expect(chosenWorkers[0].Name()).To(Equal(workerA.Name()))
				})
			})

			Context("when multiple workers satisfy the spec", func() {
				BeforeEach(func() {
					workerA.SatisfiesReturns(true)
					workerB.SatisfiesReturns(true)
					workerC.SatisfiesReturns(false)
				})

				It("succeeds and returns the first compatible worker with the container", func() {
					Expect(chooseErr).NotTo(HaveOccurred())
					Expect(len(chosenWorkers)).To(Equal(2))
					Expect(chosenWorkers[0].Name()).To(Equal(workerA.Name()))
					Expect(chosenWorkers[1].Name()).To(Equal(workerB.Name()))
				})
			})

			Context("when the worker that has the resource cache does not satisfy the spec", func() {
				BeforeEach(func() {
					workerA.SatisfiesReturns(true)
					workerB.SatisfiesReturns(true)
					workerC.SatisfiesReturns(false)

					fakeProvider.FindWorkersForResourceCacheReturns([]Worker{workerC}, nil)
				})

				It("returns empty worker list", func() {
					Expect(chooseErr).ToNot(HaveOccurred())
					Expect(chosenWorkers).To(BeEmpty())
				})
			})
		})
	})
})
