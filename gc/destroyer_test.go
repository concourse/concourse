package gc_test

import (
	"errors"

	"github.com/concourse/atc/gc"

	"code.cloudfoundry.org/lager/lagertest"
	"github.com/concourse/atc/db/dbfakes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Destroyer", func() {
	var (
		fakeContainerRepository *dbfakes.FakeContainerRepository
		fakeVolumeRepository    *dbfakes.FakeVolumeRepository
		destroyer               gc.Destroyer
	)

	BeforeEach(func() {
		fakeContainerRepository = new(dbfakes.FakeContainerRepository)
		fakeVolumeRepository = new(dbfakes.FakeVolumeRepository)

		logger = lagertest.NewTestLogger("test")

		destroyer = gc.NewDestroyer(
			logger,
			fakeContainerRepository,
			fakeVolumeRepository,
		)
	})

	Describe("Destroy Containers", func() {
		var (
			err        error
			workerName string
			handles    []string
		)

		Context("there are containers to destroy", func() {
			BeforeEach(func() {
				handles = []string{"some-handle1", "some-handle2"}
				workerName = "some-worker"

			})

			It("succeed", func() {
				fakeContainerRepository.RemoveDestroyingContainersReturns(2, nil)
				err = destroyer.DestroyContainers(workerName, handles)

				Expect(err).NotTo(HaveOccurred())
				Expect(fakeContainerRepository.RemoveDestroyingContainersCallCount()).To(Equal(1))
			})

			Context("when worker name is not provided", func() {
				It("returns an error", func() {

					err = destroyer.DestroyContainers("", handles)

					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(Equal("worker-name-must-be-provided"))
					Expect(fakeContainerRepository.RemoveDestroyingContainersCallCount()).To(Equal(0))
				})
			})
		})

		Context("there are no containers to destroy", func() {
			BeforeEach(func() {
				handles = []string{}
				workerName = "some-worker"
			})
			It("returned no error and called container repository", func() {
				err = destroyer.DestroyContainers(workerName, handles)

				Expect(err).NotTo(HaveOccurred())
				Expect(fakeContainerRepository.RemoveDestroyingContainersCallCount()).To(Equal(1))

				err = destroyer.DestroyContainers(workerName, nil)

				Expect(err).NotTo(HaveOccurred())
				Expect(fakeContainerRepository.RemoveDestroyingContainersCallCount()).To(Equal(1))
			})
		})

		Context("there is error in the container repository call", func() {
			var repoErrorString string
			BeforeEach(func() {
				handles = []string{"container_one", "container_two", "container_three"}
				workerName = "some-worker"
				repoErrorString = "I am le tired"

				fakeContainerRepository.RemoveDestroyingContainersReturns(0, errors.New(repoErrorString))
			})
			It("returns an error", func() {
				err = destroyer.DestroyContainers(workerName, handles)

				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal(repoErrorString))
				Expect(fakeContainerRepository.RemoveDestroyingContainersCallCount()).To(Equal(1))
			})
		})
	})

	Describe("Destroy Volumes", func() {
		var (
			err        error
			workerName string
			handles    []string
		)

		Context("there are volumes to destroy", func() {
			BeforeEach(func() {
				handles = []string{"some-handle1", "some-handle2"}
				workerName = "some-worker"

			})

			It("succeed", func() {
				fakeVolumeRepository.RemoveDestroyingVolumesReturns(2, nil)
				err = destroyer.DestroyVolumes(workerName, handles)

				Expect(err).NotTo(HaveOccurred())
				Expect(fakeVolumeRepository.RemoveDestroyingVolumesCallCount()).To(Equal(1))
			})

			Context("when worker name is not provided", func() {
				It("returns an error", func() {

					err = destroyer.DestroyVolumes("", handles)

					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(Equal("worker-name-must-be-provided"))
					Expect(fakeVolumeRepository.RemoveDestroyingVolumesCallCount()).To(Equal(0))
				})
			})
		})

		Context("there are no volumes to destroy", func() {
			BeforeEach(func() {
				handles = []string{}
				workerName = "some-worker"
			})
			It("returned no error and called volume repository", func() {
				err = destroyer.DestroyVolumes(workerName, handles)

				Expect(err).NotTo(HaveOccurred())
				Expect(fakeVolumeRepository.RemoveDestroyingVolumesCallCount()).To(Equal(1))

				err = destroyer.DestroyVolumes(workerName, nil)

				Expect(err).NotTo(HaveOccurred())
				Expect(fakeVolumeRepository.RemoveDestroyingVolumesCallCount()).To(Equal(1))
			})
		})

		Context("there is error in the volumes repository call", func() {
			var repoErrorString string
			BeforeEach(func() {
				handles = []string{"volume_one", "volume_two", "volume_three"}
				workerName = "some-worker"
				repoErrorString = "I am le tired"

				fakeVolumeRepository.RemoveDestroyingVolumesReturns(0, errors.New(repoErrorString))
			})
			It("returns an error", func() {
				err = destroyer.DestroyVolumes(workerName, handles)

				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal(repoErrorString))
				Expect(fakeVolumeRepository.RemoveDestroyingVolumesCallCount()).To(Equal(1))
			})
		})
	})
})
