package worker_test

import (
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"
	. "github.com/concourse/concourse/atc/worker"
	"github.com/concourse/concourse/atc/worker/workerfakes"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

//go:generate counterfeiter . ContainerPlacementStrategy

var (
	strategy ContainerPlacementStrategy

	spec    ContainerSpec
	workers []Worker

	chosenWorker Worker
	chooseErr    error

	compatibleWorkerOneCache1 *workerfakes.FakeWorker
	compatibleWorkerOneCache2 *workerfakes.FakeWorker
	compatibleWorkerTwoCaches *workerfakes.FakeWorker
	compatibleWorkerNoCaches1 *workerfakes.FakeWorker
	compatibleWorkerNoCaches2 *workerfakes.FakeWorker

	logger *lagertest.TestLogger
)

var _ = Describe("LeastContainersFoundPlacementStrategy", func() {

	Describe("Choose", func() {
		var compatibleWorker1 *workerfakes.FakeWorker
		var compatibleWorker2 *workerfakes.FakeWorker
		var compatibleWorker3 *workerfakes.FakeWorker
		JustBeforeEach(func() {
			chosenWorker, chooseErr = strategy.Choose(
				logger,
				workers,
				spec,
			)
		})

		BeforeEach(func() {
			logger = lagertest.NewTestLogger("least-containers-found-placement-test")
			strategy = NewLeastContainersFoundPlacementStrategy()
			compatibleWorker1 = new(workerfakes.FakeWorker)
			compatibleWorker2 = new(workerfakes.FakeWorker)
			compatibleWorker3 = new(workerfakes.FakeWorker)

			spec = ContainerSpec{
				ImageSpec: ImageSpec{ResourceType: "some-type"},

				TeamID: 4567,

				Inputs: []InputSource{},
			}
		})

		Context("when there is only one worker", func() {
			BeforeEach(func() {
				workers = []Worker{compatibleWorker1}
				compatibleWorker1.ActiveContainersReturns(20)
			})

			It("picks that worker", func() {
				Expect(chooseErr).ToNot(HaveOccurred())
				Expect(chosenWorker).To(Equal(compatibleWorker1))
			})
		})

		Context("when there are multiple workers", func() {
			BeforeEach(func() {
				workers = []Worker{compatibleWorker2, compatibleWorker3}

				compatibleWorker2.ActiveContainersReturns(20)
				compatibleWorker3.ActiveContainersReturns(10)
			})

			It("picks the one with least amount of containers", func() {
				Expect(chooseErr).ToNot(HaveOccurred())
				Expect(chosenWorker).To(Equal(compatibleWorker3))
			})

			Context("when there is more than one worker with the same resource utilization", func() {
				BeforeEach(func() {
					workers = []Worker{compatibleWorker1, compatibleWorker2, compatibleWorker3}
					compatibleWorker1.ActiveContainersReturns(10)
				})

				It("picks any of them", func() {
					Expect(chooseErr).ToNot(HaveOccurred())
					Expect(chosenWorker).To(Or(Equal(compatibleWorker1), Equal(compatibleWorker3)))
				})
			})
		})
	})
})

var _ = Describe("VolumeLocalityPlacementStrategy", func() {
	Describe("Choose", func() {
		JustBeforeEach(func() {
			chosenWorker, chooseErr = strategy.Choose(
				logger,
				workers,
				spec,
			)
		})

		BeforeEach(func() {
			logger = lagertest.NewTestLogger("volume-locality-placement-test")
			strategy = NewVolumeLocalityPlacementStrategy()

			fakeInput1 := new(workerfakes.FakeInputSource)
			fakeInput1AS := new(workerfakes.FakeArtifactSource)
			fakeInput1AS.VolumeOnStub = func(logger lager.Logger, worker Worker) (Volume, bool, error) {
				switch worker {
				case compatibleWorkerOneCache1, compatibleWorkerOneCache2, compatibleWorkerTwoCaches:
					return new(workerfakes.FakeVolume), true, nil
				default:
					return nil, false, nil
				}
			}
			fakeInput1.SourceReturns(fakeInput1AS)

			fakeInput2 := new(workerfakes.FakeInputSource)
			fakeInput2AS := new(workerfakes.FakeArtifactSource)
			fakeInput2AS.VolumeOnStub = func(logger lager.Logger, worker Worker) (Volume, bool, error) {
				switch worker {
				case compatibleWorkerTwoCaches:
					return new(workerfakes.FakeVolume), true, nil
				default:
					return nil, false, nil
				}
			}
			fakeInput2.SourceReturns(fakeInput2AS)

			spec = ContainerSpec{
				ImageSpec: ImageSpec{ResourceType: "some-type"},

				TeamID: 4567,

				Inputs: []InputSource{
					fakeInput1,
					fakeInput2,
				},
			}

			compatibleWorkerOneCache1 = new(workerfakes.FakeWorker)
			compatibleWorkerOneCache1.SatisfyingReturns(compatibleWorkerOneCache1, nil)

			compatibleWorkerOneCache2 = new(workerfakes.FakeWorker)
			compatibleWorkerOneCache2.SatisfyingReturns(compatibleWorkerOneCache2, nil)

			compatibleWorkerTwoCaches = new(workerfakes.FakeWorker)
			compatibleWorkerTwoCaches.SatisfyingReturns(compatibleWorkerTwoCaches, nil)

			compatibleWorkerNoCaches1 = new(workerfakes.FakeWorker)
			compatibleWorkerNoCaches1.SatisfyingReturns(compatibleWorkerNoCaches1, nil)

			compatibleWorkerNoCaches2 = new(workerfakes.FakeWorker)
			compatibleWorkerNoCaches2.SatisfyingReturns(compatibleWorkerNoCaches2, nil)
		})

		Context("with one having the most local caches", func() {
			BeforeEach(func() {
				workers = []Worker{
					compatibleWorkerOneCache1,
					compatibleWorkerTwoCaches,
					compatibleWorkerNoCaches1,
					compatibleWorkerNoCaches2,
				}
			})

			It("creates it on the worker with the most caches", func() {
				Expect(chooseErr).ToNot(HaveOccurred())
				Expect(chosenWorker).To(Equal(compatibleWorkerTwoCaches))
			})
		})

		Context("with multiple with the same amount of local caches", func() {
			BeforeEach(func() {
				workers = []Worker{
					compatibleWorkerOneCache1,
					compatibleWorkerOneCache2,
					compatibleWorkerNoCaches1,
					compatibleWorkerNoCaches2,
				}
			})

			It("creates it on a random one of the two", func() {
				Expect(chooseErr).ToNot(HaveOccurred())
				Expect(chosenWorker).To(SatisfyAny(Equal(compatibleWorkerOneCache1), Equal(compatibleWorkerOneCache2)))

				workerChoiceCounts := map[Worker]int{}

				for i := 0; i < 100; i++ {
					worker, err := strategy.Choose(
						logger,
						workers,
						spec,
					)
					Expect(err).ToNot(HaveOccurred())
					Expect(chosenWorker).To(SatisfyAny(Equal(compatibleWorkerOneCache1), Equal(compatibleWorkerOneCache2)))
					workerChoiceCounts[worker]++
				}

				Expect(workerChoiceCounts[compatibleWorkerOneCache1]).ToNot(BeZero())
				Expect(workerChoiceCounts[compatibleWorkerOneCache2]).ToNot(BeZero())
				Expect(workerChoiceCounts[compatibleWorkerNoCaches1]).To(BeZero())
				Expect(workerChoiceCounts[compatibleWorkerNoCaches2]).To(BeZero())
			})
		})

		Context("with none having any local caches", func() {
			BeforeEach(func() {
				workers = []Worker{
					compatibleWorkerNoCaches1,
					compatibleWorkerNoCaches2,
				}
			})

			It("creates it on a random one of them", func() {
				Expect(chooseErr).ToNot(HaveOccurred())
				Expect(chosenWorker).To(SatisfyAny(Equal(compatibleWorkerNoCaches1), Equal(compatibleWorkerNoCaches2)))

				workerChoiceCounts := map[Worker]int{}

				for i := 0; i < 100; i++ {
					worker, err := strategy.Choose(
						logger,
						workers,
						spec,
					)
					Expect(err).ToNot(HaveOccurred())
					Expect(chosenWorker).To(SatisfyAny(Equal(compatibleWorkerNoCaches1), Equal(compatibleWorkerNoCaches2)))
					workerChoiceCounts[worker]++
				}

				Expect(workerChoiceCounts[compatibleWorkerNoCaches1]).ToNot(BeZero())
				Expect(workerChoiceCounts[compatibleWorkerNoCaches2]).ToNot(BeZero())
			})
		})
	})
})

var _ = Describe("RandomPlacementStrategy", func() {
	Describe("Choose", func() {
		JustBeforeEach(func() {
			chosenWorker, chooseErr = strategy.Choose(
				logger,
				workers,
				spec,
			)
		})

		BeforeEach(func() {
			strategy = NewRandomPlacementStrategy()

			workers = []Worker{
				compatibleWorkerNoCaches1,
				compatibleWorkerNoCaches2,
			}
		})

		It("creates it on a random one of them", func() {
			Expect(chooseErr).ToNot(HaveOccurred())
			Expect(chosenWorker).To(SatisfyAny(Equal(compatibleWorkerNoCaches1), Equal(compatibleWorkerNoCaches2)))

			workerChoiceCounts := map[Worker]int{}

			for i := 0; i < 100; i++ {
				worker, err := strategy.Choose(
					logger,
					workers,
					spec,
				)
				Expect(err).ToNot(HaveOccurred())
				Expect(chosenWorker).To(SatisfyAny(Equal(compatibleWorkerNoCaches1), Equal(compatibleWorkerNoCaches2)))
				workerChoiceCounts[worker]++
			}

			Expect(workerChoiceCounts[compatibleWorkerNoCaches1]).ToNot(BeZero())
			Expect(workerChoiceCounts[compatibleWorkerNoCaches2]).ToNot(BeZero())
		})
	})
})
