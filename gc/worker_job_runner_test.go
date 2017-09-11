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
	var fakeWorkerPool *workerfakes.FakeClient

	var workerState chan []worker.Worker

	var pool WorkerJobRunner
	var metricFuncCallCount int

	setWorkerState := func(workers []worker.Worker) {
		// two writes guarantees that it read the workers, updated its state, and
		// then read again
		workerState <- workers
		workerState <- workers
	}

	BeforeEach(func() {
		logger := lagertest.NewTestLogger("pool")

		fakeWorkerPool = &workerfakes.FakeClient{}

		fakeWorkerA = new(workerfakes.FakeWorker)
		fakeWorkerA.NameReturns("worker-a")

		fakeWorkerB = new(workerfakes.FakeWorker)
		fakeWorkerB.NameReturns("worker-b")

		state := make(chan []worker.Worker)
		workerState = state
		fakeWorkerPool.RunningWorkersStub = func(lager.Logger) ([]worker.Worker, error) {
			return <-state, nil
		}

		metricFuncCallCount = 0

		pool = NewWorkerJobRunner(logger, fakeWorkerPool, time.Millisecond, 3, func(lager.Logger, string) {
			metricFuncCallCount++
		})

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

			Context("when the worker has reached its max-in-flight", func() {
				var stopWaiting chan struct{}

				BeforeEach(func() {
					wait := make(chan struct{})
					stopWaiting = wait

					for i := 0; i < 3; i++ {
						pool.Try(logger, "worker-a", JobFunc(func(worker.Worker) {
							<-wait
						}))
					}
				})

				It("drops any new jobs for the worker on the floor", func() {
					can := make(chan struct{}, 100)

					attempts := 0
					Consistently(func() chan struct{} {
						pool.Try(logger, "worker-a", JobFunc(func(worker.Worker) {
							can <- struct{}{}
						}))

						attempts++
						return can
					}).ShouldNot(Receive())

					Expect(metricFuncCallCount).To(Equal(attempts))
				})

				It("can run jobs on other workers", func() {
					called := make(chan struct{})

					pool.Try(logger, "worker-b", JobFunc(func(w worker.Worker) {
						defer GinkgoRecover()
						Expect(w).To(Equal(fakeWorkerB))
						close(called)
					}))

					<-called
				})

				Context("when the jobs finish", func() {
					BeforeEach(func() {
						close(stopWaiting)
					})

					It("can run more jobs", func() {
						can := make(chan struct{}, 100)

						Eventually(func() chan struct{} {
							pool.Try(logger, "worker-a", JobFunc(func(worker.Worker) {
								can <- struct{}{}
							}))

							return can
						}).Should(Receive())
					})
				})
			})

			Context("when a job of the same name is already in-flight", func() {
				var stopWaiting chan struct{}

				BeforeEach(func() {
					wait := make(chan struct{})
					stopWaiting = wait

					pool.Try(logger, "worker-a", namedJobFunc("some-name", func(worker.Worker) {
						<-wait
					}))
				})

				It("drops any new jobs of the same name on the floor", func() {
					can := make(chan struct{}, 100)

					Consistently(func() chan struct{} {
						pool.Try(logger, "worker-a", namedJobFunc("some-name", func(worker.Worker) {
							can <- struct{}{}
						}))

						return can
					}).ShouldNot(Receive())
				})

				It("can run jobs with other names", func() {
					called := make(chan struct{})

					pool.Try(logger, "worker-a", namedJobFunc("some-other-name", func(w worker.Worker) {
						defer GinkgoRecover()
						Expect(w).To(Equal(fakeWorkerA))
						close(called)
					}))

					<-called
				})

				It("can run jobs with no name", func() {
					called := make(chan struct{})

					pool.Try(logger, "worker-a", JobFunc(func(w worker.Worker) {
						defer GinkgoRecover()
						Expect(w).To(Equal(fakeWorkerA))
						close(called)
					}))

					<-called
				})

				Context("when the job finishes", func() {
					BeforeEach(func() {
						close(stopWaiting)
					})

					It("can run it again", func() {
						can := make(chan struct{}, 100)

						Eventually(func() chan struct{} {
							pool.Try(logger, "worker-a", namedJobFunc("some-name", func(worker.Worker) {
								can <- struct{}{}
							}))

							return can
						}).Should(Receive())
					})
				})
			})
		})
	})
})

type njf struct {
	name string
	f    func(worker.Worker)
}

func namedJobFunc(name string, f func(worker.Worker)) Job {
	return njf{
		name: name,
		f:    f,
	}
}

func (njf njf) Name() string { return njf.name }

func (njf njf) Run(w worker.Worker) {
	njf.f(w)
}
