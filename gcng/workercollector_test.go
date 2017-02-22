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
		workerCollector     gcng.Collector
		fakeWorkerLifecycle *dbngfakes.FakeWorkerLifecycle
	)

	BeforeEach(func() {
		logger := lagertest.NewTestLogger("volume-collector")
		fakeWorkerLifecycle = new(dbngfakes.FakeWorkerLifecycle)

		workerCollector = gcng.NewWorkerCollector(
			logger,
			fakeWorkerLifecycle,
		)

		fakeWorkerLifecycle.StallUnresponsiveWorkersReturns(nil, nil)
		fakeWorkerLifecycle.DeleteFinishedRetiringWorkersReturns(nil, nil)
		fakeWorkerLifecycle.LandFinishedLandingWorkersReturns(nil, nil)
	})

	Describe("Run", func() {
		It("tells the worker factory to expired stalled workers", func() {
			err := workerCollector.Run()
			Expect(err).NotTo(HaveOccurred())

			Expect(fakeWorkerLifecycle.StallUnresponsiveWorkersCallCount()).To(Equal(1))
		})

		It("tells the worker factory to delete finished retiring workers", func() {
			err := workerCollector.Run()
			Expect(err).NotTo(HaveOccurred())

			Expect(fakeWorkerLifecycle.DeleteFinishedRetiringWorkersCallCount()).To(Equal(1))
		})

		It("tells the worker factory to land finished landing workers", func() {
			err := workerCollector.Run()
			Expect(err).NotTo(HaveOccurred())

			Expect(fakeWorkerLifecycle.LandFinishedLandingWorkersCallCount()).To(Equal(1))
		})

		It("returns an error if stalling unresponsive workers fails", func() {
			returnedErr := errors.New("some-error")
			fakeWorkerLifecycle.StallUnresponsiveWorkersReturns(nil, returnedErr)

			err := workerCollector.Run()
			Expect(err).To(MatchError(returnedErr))
		})

		It("returns an error if deleting finished retiring workers fails", func() {
			returnedErr := errors.New("some-error")
			fakeWorkerLifecycle.DeleteFinishedRetiringWorkersReturns(nil, returnedErr)

			err := workerCollector.Run()
			Expect(err).To(MatchError(returnedErr))
		})

		It("returns an error if landing finished landing workers fails", func() {
			returnedErr := errors.New("some-error")
			fakeWorkerLifecycle.LandFinishedLandingWorkersReturns(nil, returnedErr)

			err := workerCollector.Run()
			Expect(err).To(MatchError(returnedErr))
		})
	})
})
