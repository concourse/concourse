package worker_test

import (
	"fmt"
	"sync/atomic"
	"time"

	"code.cloudfoundry.org/lager/v3"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/metric"
	"github.com/concourse/concourse/atc/runtime"
	"github.com/concourse/concourse/atc/worker"
	grt "github.com/concourse/concourse/atc/worker/gardenruntime/gardenruntimetest"
	"github.com/concourse/concourse/atc/worker/workertest"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Pool", func() {
	Describe("FindOrSelectWorker", func() {
		Test("find a worker with an existing container", func() {
			concurrentId := GinkgoParallelProcess()
			scenario := Setup(
				workertest.WithWorkers(
					grt.NewWorker(fmt.Sprintf("worker1-%d", concurrentId)),
					grt.NewWorker(fmt.Sprintf("worker2-%d", concurrentId)).
						WithDBContainersInState(grt.Creating, "my-container"),
					grt.NewWorker(fmt.Sprintf("worker3-%d", concurrentId)),
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

			Expect(worker.Name()).To(Equal(fmt.Sprintf("worker2-%d", concurrentId)))
		})

		Test("selects a worker when container owner has no worker", func() {
			concurrentId := GinkgoParallelProcess()
			scenario := Setup(
				workertest.WithWorkers(
					grt.NewWorker(fmt.Sprintf("worker1-%d", concurrentId)),
					grt.NewWorker(fmt.Sprintf("worker2-%d", concurrentId)),
					grt.NewWorker(fmt.Sprintf("worker3-%d", concurrentId)),
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

			Expect(worker.Name()).To(BeOneOf(fmt.Sprintf("worker1-%d", concurrentId), fmt.Sprintf("worker2-%d", concurrentId), fmt.Sprintf("worker3-%d", concurrentId)))
		})

		Test("follows the strategy for selecting a worker", func() {
			concurrentId := GinkgoParallelProcess()
			scenario := Setup(
				workertest.WithBasicJob(),
				workertest.WithWorkers(
					grt.NewWorker(fmt.Sprintf("worker1-%d", concurrentId)).
						WithJobBuildContainerCreatedInDBAndGarden().
						WithJobBuildContainerCreatedInDBAndGarden(),
					grt.NewWorker(fmt.Sprintf("worker2-%d", concurrentId)).
						WithJobBuildContainerCreatedInDBAndGarden(),
					grt.NewWorker(fmt.Sprintf("worker3-%d", concurrentId)),
				),
			)

			strategy, _, _, err := worker.NewPlacementStrategy(worker.PlacementOptions{
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

			Expect(worker.Name()).To(Equal(fmt.Sprintf("worker3-%d", concurrentId)))
		})

		Test("selects a new worker when owning worker is incompatible", func() {
			concurrentId := GinkgoParallelProcess()
			scenario := Setup(
				workertest.WithWorkers(
					grt.NewWorker(fmt.Sprintf("worker1-%d", concurrentId)),
					grt.NewWorker(fmt.Sprintf("worker2-%d", concurrentId)).
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

			Expect(worker.Name()).To(Equal(fmt.Sprintf("worker1-%d", concurrentId)))
		})

		Test("selects a new worker when owning worker is not running", func() {
			concurrentId := GinkgoParallelProcess()
			scenario := Setup(
				workertest.WithWorkers(
					grt.NewWorker(fmt.Sprintf("worker1-%d", concurrentId)),
					grt.NewWorker(fmt.Sprintf("worker2-%d", concurrentId)).
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

			Expect(worker.Name()).To(Equal(fmt.Sprintf("worker1-%d", concurrentId)))
		})

		Test("filters out incompatible workers by resource type", func() {
			concurrentId := GinkgoParallelProcess()
			scenario := Setup(
				workertest.WithWorkers(
					grt.NewWorker(fmt.Sprintf("worker1-%d", concurrentId)),
					grt.NewWorker(fmt.Sprintf("worker2-%d", concurrentId)),
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
			concurrentId := GinkgoParallelProcess()
			scenario := Setup(
				workertest.WithWorkers(
					grt.NewWorker(fmt.Sprintf("worker1-%d", concurrentId)),
					grt.NewWorker(fmt.Sprintf("worker2-%d", concurrentId)),
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
			concurrentId := GinkgoParallelProcess()
			scenario := Setup(
				workertest.WithWorkers(
					grt.NewWorker(fmt.Sprintf("worker1-%d", concurrentId)).WithTags("A", "C"),
					grt.NewWorker(fmt.Sprintf("worker2-%d", concurrentId)).WithTags("B", "C"),
					grt.NewWorker(fmt.Sprintf("worker3-%d", concurrentId)),
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
			concurrentId := GinkgoParallelProcess()
			scenario := Setup(
				workertest.WithTeam("team"),
				workertest.WithWorkers(
					grt.NewWorker(fmt.Sprintf("worker1-%d", concurrentId)).WithTeam("team"),
					grt.NewWorker(fmt.Sprintf("worker2-%d", concurrentId)),
					grt.NewWorker(fmt.Sprintf("worker3-%d", concurrentId)),
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

			Expect(worker.Name()).To(Equal(fmt.Sprintf("worker1-%d", concurrentId)))
		})

		Test("considers general workers when all team workers are incompatible", func() {
			concurrentId := GinkgoParallelProcess()
			scenario := Setup(
				workertest.WithTeam("team"),
				workertest.WithWorkers(
					grt.NewWorker(fmt.Sprintf("worker1-%d", concurrentId)).WithTeam("team").WithPlatform("dummy"),
					grt.NewWorker(fmt.Sprintf("worker2-%d", concurrentId)),
					grt.NewWorker(fmt.Sprintf("worker3-%d", concurrentId)),
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

			Expect(worker.Name()).To(BeOneOf(fmt.Sprintf("worker2-%d", concurrentId), fmt.Sprintf("worker3-%d", concurrentId)))
		})

		Test("no worker satisfies strategy", func() {
			concurrentId := GinkgoParallelProcess()
			scenario := Setup(
				workertest.WithWorkers(
					grt.NewWorker(fmt.Sprintf("worker1-%d", concurrentId)).
						WithActiveTasks(1),
					grt.NewWorker(fmt.Sprintf("worker2-%d", concurrentId)).
						WithActiveTasks(1),
				),
			)

			strategy, _, _, err := worker.NewPlacementStrategy(worker.PlacementOptions{
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
				strategy.Release(logger, scenario.Worker(fmt.Sprintf("worker1-%d", concurrentId)).DBWorker(), taskSpec)
				worker := <-workerCh
				Expect(worker.Name()).To(Equal(fmt.Sprintf("worker1-%d", concurrentId)))
			})
		})
	})

	Describe("FindResourceCacheVolume", func() {
		Test("finds a resource cache volume among multiple workers", func() {
			concurrentId := GinkgoParallelProcess()
			scenario := Setup(
				workertest.WithWorkers(
					grt.NewWorker(fmt.Sprintf("worker1-%d", concurrentId)).
						WithVolumesCreatedInDBAndBaggageclaim(
							grt.NewVolume("resource-cache-1"),
						),
					grt.NewWorker(fmt.Sprintf("worker2-%d", concurrentId)).
						WithVolumesCreatedInDBAndBaggageclaim(
							grt.NewVolume("resource-cache-2"),
						),
					grt.NewWorker(fmt.Sprintf("worker3-%d", concurrentId)),
				),
			)
			resourceCache := scenario.FindOrCreateResourceCache(fmt.Sprintf("worker1-%d", concurrentId))

			_, err := scenario.WorkerVolume(fmt.Sprintf("worker1-%d", concurrentId), "resource-cache-1").InitializeResourceCache(ctx, resourceCache)
			Expect(err).ToNot(HaveOccurred())

			_, err = scenario.WorkerVolume(fmt.Sprintf("worker2-%d", concurrentId), "resource-cache-2").InitializeResourceCache(ctx, resourceCache)
			Expect(err).ToNot(HaveOccurred())

			cacheVolume, found, err := scenario.Pool.FindResourceCacheVolume(ctx, 0, resourceCache, worker.Spec{}, time.Now())
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())
			Expect(cacheVolume.Handle()).To(BeOneOf("resource-cache-1", "resource-cache-2"))
		})

		Test("skips over workers with volume missing", func() {
			concurrentId := GinkgoParallelProcess()
			scenario := Setup(
				workertest.WithWorkers(
					grt.NewWorker(fmt.Sprintf("worker1-%d", concurrentId)).
						WithVolumesCreatedInDBAndBaggageclaim(
							grt.NewVolume("resource-cache-1"),
						),
					grt.NewWorker(fmt.Sprintf("worker2-%d", concurrentId)).
						WithVolumesCreatedInDBAndBaggageclaim(
							grt.NewVolume("resource-cache-2"),
						),
					grt.NewWorker(fmt.Sprintf("worker3-%d", concurrentId)),
				),
			)
			resourceCache := scenario.FindOrCreateResourceCache(fmt.Sprintf("worker1-%d", concurrentId))

			_, err := scenario.WorkerVolume(fmt.Sprintf("worker1-%d", concurrentId), "resource-cache-1").InitializeResourceCache(ctx, resourceCache)
			Expect(err).ToNot(HaveOccurred())

			_, err = scenario.WorkerVolume(fmt.Sprintf("worker2-%d", concurrentId), "resource-cache-2").InitializeResourceCache(ctx, resourceCache)
			Expect(err).ToNot(HaveOccurred())

			By("destroying one of the worker's resource cache volumes", func() {
				_, err := scenario.WorkerVolume(fmt.Sprintf("worker1-%d", concurrentId), "resource-cache-1").DBVolume().Destroying()
				Expect(err).ToNot(HaveOccurred())
			})

			cacheVolume, found, err := scenario.Pool.FindResourceCacheVolume(ctx, 0, resourceCache, worker.Spec{}, time.Now())
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())
			Expect(cacheVolume.Handle()).To(Equal("resource-cache-2"))
		})

		Test("skips over stalled workers", func() {
			concurrentId := GinkgoParallelProcess()
			scenario := Setup(
				workertest.WithWorkers(
					grt.NewWorker(fmt.Sprintf("worker1-%d", concurrentId)).
						WithVolumesCreatedInDBAndBaggageclaim(
							grt.NewVolume("resource-cache-1"),
						).
						WithState(db.WorkerStateStalled),
				),
			)
			resourceCache := scenario.FindOrCreateResourceCache(fmt.Sprintf("worker1-%d", concurrentId))

			_, err := scenario.WorkerVolume(fmt.Sprintf("worker1-%d", concurrentId), "resource-cache-1").InitializeResourceCache(ctx, resourceCache)
			Expect(err).ToNot(HaveOccurred())

			_, found, err := scenario.Pool.FindResourceCacheVolume(ctx, 0, resourceCache, worker.Spec{}, time.Now())
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeFalse())
		})
	})

	Describe("FindResourceCacheVolumeOnWorker", func() {
		Test("finds a resource cache volume on a worker", func() {
			concurrentId := GinkgoParallelProcess()
			scenario := Setup(
				workertest.WithWorkers(
					grt.NewWorker(fmt.Sprintf("worker1-%d", concurrentId)).
						WithVolumesCreatedInDBAndBaggageclaim(
							grt.NewVolume("resource-cache-1"),
						),
					grt.NewWorker(fmt.Sprintf("worker2-%d", concurrentId)).
						WithVolumesCreatedInDBAndBaggageclaim(
							grt.NewVolume("resource-cache-2"),
						),
					grt.NewWorker(fmt.Sprintf("worker3-%d", concurrentId)),
				),
			)
			resourceCache := scenario.FindOrCreateResourceCache(fmt.Sprintf("worker1-%d", concurrentId))

			_, err := scenario.WorkerVolume(fmt.Sprintf("worker1-%d", concurrentId), "resource-cache-1").InitializeResourceCache(ctx, resourceCache)
			Expect(err).ToNot(HaveOccurred())

			_, err = scenario.WorkerVolume(fmt.Sprintf("worker2-%d", concurrentId), "resource-cache-2").InitializeResourceCache(ctx, resourceCache)
			Expect(err).ToNot(HaveOccurred())

			cacheVolume, found, err := scenario.Pool.FindResourceCacheVolumeOnWorker(ctx, resourceCache, worker.Spec{}, fmt.Sprintf("worker1-%d", concurrentId), time.Now())
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())
			Expect(cacheVolume.Handle()).To(Equal("resource-cache-1"))
		})

		Test("ignores invalid worker names", func() {
			concurrentId := GinkgoParallelProcess()
			scenario := Setup(
				workertest.WithWorkers(
					grt.NewWorker(fmt.Sprintf("worker1-%d", concurrentId)).
						WithVolumesCreatedInDBAndBaggageclaim(
							grt.NewVolume("resource-cache-1"),
						),
				),
			)

			resourceCache := scenario.FindOrCreateResourceCache(fmt.Sprintf("worker1-%d", concurrentId))

			_, err := scenario.WorkerVolume(fmt.Sprintf("worker1-%d", concurrentId), "resource-cache-1").InitializeResourceCache(ctx, resourceCache)
			Expect(err).ToNot(HaveOccurred())

			_, found, err := scenario.Pool.FindResourceCacheVolumeOnWorker(ctx, resourceCache, worker.Spec{}, "invalid-worker", time.Now())

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

			_, err := scenario.WorkerVolume("stalled-worker", "resource-cache-1").InitializeResourceCache(ctx, resourceCache)
			Expect(err).ToNot(HaveOccurred())

			_, found, err := scenario.Pool.FindResourceCacheVolumeOnWorker(ctx, resourceCache, worker.Spec{}, "stalled-worker", time.Now())
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
