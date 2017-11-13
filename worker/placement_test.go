package worker_test

import (
	. "github.com/concourse/atc/worker"
	"github.com/concourse/atc/worker/workerfakes"

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
)

var _ = Describe("VolumeLocalityPlacementStrategy", func() {
	Describe("Choose", func() {
		JustBeforeEach(func() {
			chosenWorker, chooseErr = strategy.Choose(
				workers,
				spec,
			)
		})

		BeforeEach(func() {
			strategy = NewVolumeLocalityPlacementStrategy()

			fakeInput1 := new(workerfakes.FakeInputSource)
			fakeInput1AS := new(workerfakes.FakeArtifactSource)
			fakeInput1AS.VolumeOnStub = func(worker Worker) (Volume, bool, error) {
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
			fakeInput2AS.VolumeOnStub = func(worker Worker) (Volume, bool, error) {
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
