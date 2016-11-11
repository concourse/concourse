package gcng_test

import (
	"code.cloudfoundry.org/lager/lagertest"
	"github.com/concourse/atc/gcng"

	"errors"

	"github.com/concourse/atc/dbng/dbngfakes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("WorkerCollector", func() {
	var (
		workerCollector gcng.WorkerCollector

		fakeWorkerFactory *dbngfakes.FakeWorkerFactory
	)

	BeforeEach(func() {
		logger := lagertest.NewTestLogger("volume-collector")
		fakeWorkerFactory = new(dbngfakes.FakeWorkerFactory)

		workerCollector = gcng.NewWorkerCollector(
			logger,
			fakeWorkerFactory,
		)

		fakeWorkerFactory.StallUnresponsiveWorkersReturns(nil, nil)
		fakeWorkerFactory.DeleteFinishedLandingWorkersReturns(nil)
	})

	Describe("Run", func() {
		It("tells the worker factory to expired stalled workers", func() {
			err := workerCollector.Run()
			Expect(err).NotTo(HaveOccurred())

			Expect(fakeWorkerFactory.StallUnresponsiveWorkersCallCount()).To(Equal(1))
		})

		It("tells the worker factory to delete finished landing workers", func() {
			err := workerCollector.Run()
			Expect(err).NotTo(HaveOccurred())

			Expect(fakeWorkerFactory.DeleteFinishedLandingWorkersCallCount()).To(Equal(1))
		})

		It("returns an error if stalling unresponsive workers fails", func() {
			returnedErr := errors.New("some-error")
			fakeWorkerFactory.StallUnresponsiveWorkersReturns(nil, returnedErr)

			err := workerCollector.Run()
			Expect(err).To(MatchError(returnedErr))
		})

		It("returns an error if deleting finished landing workers fails", func() {
			returnedErr := errors.New("some-error")
			fakeWorkerFactory.DeleteFinishedLandingWorkersReturns(returnedErr)

			err := workerCollector.Run()
			Expect(err).To(MatchError(returnedErr))
		})
	})
})
