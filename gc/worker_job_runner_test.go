package gc_test

import (
	"time"

	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"
	. "github.com/concourse/atc/gc"
	"github.com/concourse/atc/worker"
	"github.com/concourse/atc/worker/workerfakes"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("WorkerJobRunner", func() {
	var fakeWorkerA *workerfakes.FakeWorker
	var fakeWorkerB *workerfakes.FakeWorker
	var fakeWorkerProvider *workerfakes.FakeWorkerProvider

	var workerState chan []worker.Worker

	var pool WorkerJobRunner

	setWorkerState := func(workers []worker.Worker) {
		// two writes guarantees that it read the workers, updated its state, and
		// then read again
		workerState <- workers
		workerState <- workers
	}

	BeforeEach(func() {
		logger := lagertest.NewTestLogger("pool")

		fakeWorkerProvider = &workerfakes.FakeWorkerProvider{}

		fakeWorkerA = new(workerfakes.FakeWorker)
		fakeWorkerA.NameReturns("worker-a")

		fakeWorkerB = new(workerfakes.FakeWorker)
		fakeWorkerB.NameReturns("worker-b")

		state := make(chan []worker.Worker)
		workerState = state
		fakeWorkerProvider.RunningWorkersStub = func(lager.Logger) ([]worker.Worker, error) {
			return <-state, nil
		}

		pool = NewWorkerJobRunner(logger, fakeWorkerProvider, time.Millisecond)

		setWorkerState([]worker.Worker{fakeWorkerA, fakeWorkerB})
	})

	Context("Try", func() {
		It("does nothing if the worker doesn't exist", func() {
			pool.Try(logger, "some-bogus-name", JobFunc(func(worker.Worker) {
				defer GinkgoRecover()
				Fail("should not be called")
			}))
		})

		Context("when the worker exists", func() {
			It("calls the job with the correct worker", func() {
				called := make(chan struct{})

				pool.Try(logger, "worker-a", JobFunc(func(w worker.Worker) {
					defer GinkgoRecover()
					Expect(w).To(Equal(fakeWorkerA))
					close(called)
				}))

				<-called
			})

			Context("when a worker disappears", func() {
				BeforeEach(func() {
					setWorkerState([]worker.Worker{fakeWorkerB})
				})

				It("no longer runs jobs on the worker", func() {
					pool.Try(logger, "worker-a", JobFunc(func(worker.Worker) {
						defer GinkgoRecover()
						Fail("should not be called")
					}))
				})
			})
		})
	})
})
