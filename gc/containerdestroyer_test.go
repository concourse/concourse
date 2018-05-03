package gc_test

import (
	"errors"

	"github.com/concourse/atc/gc"

	"code.cloudfoundry.org/lager/lagertest"
	"github.com/concourse/atc/db/dbfakes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("ContainerDestroyer", func() {
	var (
		fakeContainerRepository *dbfakes.FakeContainerRepository
		logger                  *lagertest.TestLogger
		destroyer               gc.ContainerDestroyer
	)

	BeforeEach(func() {
		fakeContainerRepository = new(dbfakes.FakeContainerRepository)

		logger = lagertest.NewTestLogger("test")

		destroyer = gc.NewContainerDestroyer(
			logger,
			fakeContainerRepository,
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
				err = destroyer.Destroy(workerName, handles)

				Expect(err).NotTo(HaveOccurred())
				Expect(fakeContainerRepository.RemoveDestroyingContainersCallCount()).To(Equal(1))
			})

			Context("when worker name is not provided", func() {
				It("returns an error", func() {

					err = destroyer.Destroy("", handles)

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
				err = destroyer.Destroy(workerName, handles)

				Expect(err).NotTo(HaveOccurred())
				Expect(fakeContainerRepository.RemoveDestroyingContainersCallCount()).To(Equal(1))

				err = destroyer.Destroy(workerName, nil)

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
				err = destroyer.Destroy(workerName, handles)

				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal(repoErrorString))
				Expect(fakeContainerRepository.RemoveDestroyingContainersCallCount()).To(Equal(1))
			})
		})
	})
})
