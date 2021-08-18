package worker_test

import (
	"sync/atomic"
	"time"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/metric"
	"github.com/concourse/concourse/atc/runtime"
	"github.com/concourse/concourse/atc/worker"
	grt "github.com/concourse/concourse/atc/worker/gardenruntime/gardenruntimetest"
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
				ctx,
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
				ctx,
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
				workertest.WithBasicJob(),
				workertest.WithWorkers(
					grt.NewWorker("worker1").
						WithJobBuildContainerCreatedInDBAndGarden().
						WithJobBuildContainerCreatedInDBAndGarden(),
					grt.NewWorker("worker2").
						WithJobBuildContainerCreatedInDBAndGarden(),
					grt.NewWorker("worker3"),
				),
			)

			strategy, err := worker.NewPlacementStrategy(worker.PlacementOptions{
				Strategies: []string{"fewest-build-containers"},
			})
			Expect(err).ToNot(HaveOccurred())

			worker, err := scenario.Pool.FindOrSelectWorker(
				ctx,
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
				ctx,
				db.NewFixedHandleContainerOwner("my-container"),
				runtime.ContainerSpec{},
				worker.Spec{},
				nil,
				nil,
			)
			Expect(err).ToNot(HaveOccurred())

			Expect(worker.Name()).To(Equal("worker1"))
		})

		Test("selects a new worker when owning worker is not running", func() {
			scenario := Setup(
				workertest.WithWorkers(
					grt.NewWorker("worker1"),
					grt.NewWorker("worker2").
						WithContainersCreatedInDBAndGarden(
							grt.NewContainer("my-container"),
						).
						WithState(db.WorkerStateStalled),
				),
			)

			worker, err := scenario.Pool.FindOrSelectWorker(
				ctx,
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
				ctx,
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
				ctx,
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
				ctx,
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
				ctx,
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
				ctx,
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
						WithActiveTasks(1),
					grt.NewWorker("worker2").
						WithActiveTasks(1),
				),
			)

			strategy, err := worker.NewPlacementStrategy(worker.PlacementOptions{
				Strategies:              []string{"limit-active-tasks"},
				MaxActiveTasksPerWorker: 1,
			})
			Expect(err).ToNot(HaveOccurred())

			taskSpec := runtime.ContainerSpec{Type: db.ContainerTypeTask}

			workerCh := make(chan runtime.Worker)

			var callbackInvocations int32
			callback := PoolCallback{
				waitingForWorker: func() { atomic.AddInt32(&callbackInvocations, 1) },
			}

			By("selecting a worker when there are no satisfiable workers", func() {
				worker.PollingInterval = 10 * time.Millisecond

				go func() {
					defer GinkgoRecover()

					worker, err := scenario.Pool.FindOrSelectWorker(
						ctx,
						db.NewFixedHandleContainerOwner("my-container"),
						taskSpec,
						worker.Spec{TeamID: 123},
						strategy,
						callback,
					)
					Expect(err).ToNot(HaveOccurred())

					workerCh <- worker
				}()
			})

			By("validating that the step is marked as waiting", func() {
				callbackCount := func() int32 { return atomic.LoadInt32(&callbackInvocations) }
				metricCount := func() float64 {
					labels := metric.StepsWaitingLabels{
						TeamId: "123",
						Type:   string(db.ContainerTypeTask),
					}
					return metric.Metrics.StepsWaiting[labels].Max()
				}
				Eventually(callbackCount).Should(Equal(int32(1)))
				Eventually(metricCount).Should(BeNumerically("~", 1))

				By("validating the step is only marked once", func() {
					Consistently(callbackCount).Should(Equal(int32(1)))
					Consistently(metricCount).Should(BeNumerically("~", 1))
				})
			})

			By("freeing up a worker", func() {
				strategy.Release(logger, scenario.Worker("worker1").DBWorker(), taskSpec)
				worker := <-workerCh
				Expect(worker.Name()).To(Equal("worker1"))
			})
		})
	})

	Describe("FindResourceCacheVolume", func() {
		Test("finds a resource cache volume among multiple workers", func() {
			scenario := Setup(
				workertest.WithWorkers(
					grt.NewWorker("worker1").
						WithVolumesCreatedInDBAndBaggageclaim(
							grt.NewVolume("resource-cache-1"),
						),
					grt.NewWorker("worker2").
						WithVolumesCreatedInDBAndBaggageclaim(
							grt.NewVolume("resource-cache-2"),
						),
					grt.NewWorker("worker3"),
				),
			)
			resourceCache := scenario.FindOrCreateResourceCache("worker1")

			err := scenario.WorkerVolume("worker1", "resource-cache-1").InitializeResourceCache(logger, resourceCache)
			Expect(err).ToNot(HaveOccurred())

			err = scenario.WorkerVolume("worker2", "resource-cache-2").InitializeResourceCache(logger, resourceCache)
			Expect(err).ToNot(HaveOccurred())

			cacheVolume, found, err := scenario.Pool.FindResourceCacheVolume(logger, 0, resourceCache, worker.Spec{})
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())
			Expect(cacheVolume.Handle()).To(BeOneOf("resource-cache-1", "resource-cache-2"))
		})

		Test("skips over workers with volume missing", func() {
			scenario := Setup(
				workertest.WithWorkers(
					grt.NewWorker("worker1").
						WithVolumesCreatedInDBAndBaggageclaim(
							grt.NewVolume("resource-cache-1"),
						),
					grt.NewWorker("worker2").
						WithVolumesCreatedInDBAndBaggageclaim(
							grt.NewVolume("resource-cache-2"),
						),
					grt.NewWorker("worker3"),
				),
			)
			resourceCache := scenario.FindOrCreateResourceCache("worker1")

			err := scenario.WorkerVolume("worker1", "resource-cache-1").InitializeResourceCache(logger, resourceCache)
			Expect(err).ToNot(HaveOccurred())

			err = scenario.WorkerVolume("worker2", "resource-cache-2").InitializeResourceCache(logger, resourceCache)
			Expect(err).ToNot(HaveOccurred())

			By("destroying one of the worker's resource cache volumes", func() {
				_, err := scenario.WorkerVolume("worker1", "resource-cache-1").DBVolume().Destroying()
				Expect(err).ToNot(HaveOccurred())
			})

			cacheVolume, found, err := scenario.Pool.FindResourceCacheVolume(logger, 0, resourceCache, worker.Spec{})
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())
			Expect(cacheVolume.Handle()).To(Equal("resource-cache-2"))
		})

		Test("skips over stalled workers", func() {
			scenario := Setup(
				workertest.WithWorkers(
					grt.NewWorker("worker1").
						WithVolumesCreatedInDBAndBaggageclaim(
							grt.NewVolume("resource-cache-1"),
						).
						WithState(db.WorkerStateStalled),
				),
			)
			resourceCache := scenario.FindOrCreateResourceCache("worker1")

			err := scenario.WorkerVolume("worker1", "resource-cache-1").InitializeResourceCache(logger, resourceCache)
			Expect(err).ToNot(HaveOccurred())

			_, found, err := scenario.Pool.FindResourceCacheVolume(logger, 0, resourceCache, worker.Spec{})
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeFalse())
		})
	})

	Describe("FindResourceCacheVolumeOnWorker", func() {
		Test("finds a resource cache volume on a worker", func() {
			scenario := Setup(
				workertest.WithWorkers(
					grt.NewWorker("worker1").
						WithVolumesCreatedInDBAndBaggageclaim(
							grt.NewVolume("resource-cache-1"),
						),
					grt.NewWorker("worker2").
						WithVolumesCreatedInDBAndBaggageclaim(
							grt.NewVolume("resource-cache-2"),
						),
					grt.NewWorker("worker3"),
				),
			)
			resourceCache := scenario.FindOrCreateResourceCache("worker1")

			err := scenario.WorkerVolume("worker1", "resource-cache-1").InitializeResourceCache(logger, resourceCache)
			Expect(err).ToNot(HaveOccurred())

			err = scenario.WorkerVolume("worker2", "resource-cache-2").InitializeResourceCache(logger, resourceCache)
			Expect(err).ToNot(HaveOccurred())

			cacheVolume, found, err := scenario.Pool.FindResourceCacheVolumeOnWorker(logger, resourceCache, worker.Spec{}, "worker1")
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())
			Expect(cacheVolume.Handle()).To(Equal("resource-cache-1"))
		})

		Test("ignores invalid worker names", func() {
			scenario := Setup(
				workertest.WithWorkers(
					grt.NewWorker("worker1").
						WithVolumesCreatedInDBAndBaggageclaim(
							grt.NewVolume("resource-cache-1"),
						),
				),
			)

			resourceCache := scenario.FindOrCreateResourceCache("worker1")

			err := scenario.WorkerVolume("worker1", "resource-cache-1").InitializeResourceCache(logger, resourceCache)
			Expect(err).ToNot(HaveOccurred())

			_, found, err := scenario.Pool.FindResourceCacheVolumeOnWorker(logger, resourceCache, worker.Spec{}, "invalid-worker")

			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeFalse())
		})

		Test("ignores stalled workers", func() {
			scenario := Setup(
				workertest.WithWorkers(
					grt.NewWorker("stalled-worker").
						WithVolumesCreatedInDBAndBaggageclaim(
							grt.NewVolume("resource-cache-1"),
						).
						WithState(db.WorkerStateStalled),
				),
			)
			resourceCache := scenario.FindOrCreateResourceCache("stalled-worker")

			err := scenario.WorkerVolume("stalled-worker", "resource-cache-1").InitializeResourceCache(logger, resourceCache)
			Expect(err).ToNot(HaveOccurred())

			_, found, err := scenario.Pool.FindResourceCacheVolumeOnWorker(logger, resourceCache, worker.Spec{}, "stalled-worker")
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeFalse())
		})
	})
})

type PoolCallback struct {
	waitingForWorker func()
}

func (p PoolCallback) WaitingForWorker(_ lager.Logger) {
	p.waitingForWorker()
}
