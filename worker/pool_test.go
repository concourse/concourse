package worker_test

import (
	"errors"

	garden "github.com/cloudfoundry-incubator/garden/api"
	. "github.com/concourse/atc/worker"
	"github.com/concourse/atc/worker/fakes"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Pool", func() {
	var (
		fakeProvider *fakes.FakeWorkerProvider

		pool Client
	)

	BeforeEach(func() {
		fakeProvider = new(fakes.FakeWorkerProvider)

		pool = NewPool(fakeProvider)
	})

	Describe("Create", func() {
		var (
			spec garden.ContainerSpec

			createdContainer Container
			createErr        error
		)

		BeforeEach(func() {
			spec = garden.ContainerSpec{
				Handle: "some-handle",
			}
		})

		JustBeforeEach(func() {
			createdContainer, createErr = pool.Create(spec)
		})

		Context("with multiple workers", func() {
			var (
				workerA *fakes.FakeWorker
				workerB *fakes.FakeWorker

				fakeContainer *fakes.FakeContainer
			)

			BeforeEach(func() {
				workerA = new(fakes.FakeWorker)
				workerB = new(fakes.FakeWorker)

				workerA.ActiveContainersReturns(3)
				workerB.ActiveContainersReturns(2)

				fakeContainer = new(fakes.FakeContainer)
				workerA.CreateReturns(fakeContainer, nil)
				workerB.CreateReturns(fakeContainer, nil)

				fakeProvider.WorkersReturns([]Worker{workerA, workerB}, nil)
			})

			It("succeeds", func() {
				Ω(createErr).ShouldNot(HaveOccurred())
			})

			It("returns the created container", func() {
				Ω(createdContainer).Should(Equal(fakeContainer))
			})

			It("creates using a random worker", func() {
				for i := 1; i < 100; i++ { // account for initial create in JustBefore
					createdContainer, createErr := pool.Create(spec)
					Ω(createErr).ShouldNot(HaveOccurred())
					Ω(createdContainer).Should(Equal(fakeContainer))
				}

				Ω(workerA.CreateCallCount()).Should(BeNumerically("~", workerB.CreateCallCount(), 50))
			})

			Context("when creating the container fails", func() {
				disaster := errors.New("nope")

				BeforeEach(func() {
					workerA.CreateReturns(nil, disaster)
					workerB.CreateReturns(nil, disaster)
				})

				It("returns the error", func() {
					Ω(createErr).Should(Equal(disaster))
				})
			})
		})

		Context("with no workers", func() {
			BeforeEach(func() {
				fakeProvider.WorkersReturns([]Worker{}, nil)
			})

			It("returns ErrNoWorkers", func() {
				Ω(createErr).Should(Equal(ErrNoWorkers))
			})
		})

		Context("when getting the workers fails", func() {
			disaster := errors.New("nope")

			BeforeEach(func() {
				fakeProvider.WorkersReturns(nil, disaster)
			})

			It("returns the error", func() {
				Ω(createErr).Should(Equal(disaster))
			})
		})
	})

	Describe("Lookup", func() {
		var (
			handle string

			foundContainer Container
			lookupErr      error
		)

		BeforeEach(func() {
			handle = "some-handle"
		})

		JustBeforeEach(func() {
			foundContainer, lookupErr = pool.Lookup(handle)
		})

		Context("with multiple workers", func() {
			var (
				workerA *fakes.FakeWorker
				workerB *fakes.FakeWorker

				fakeContainer *fakes.FakeContainer
			)

			BeforeEach(func() {
				workerA = new(fakes.FakeWorker)
				workerB = new(fakes.FakeWorker)

				workerA.ActiveContainersReturns(3)
				workerB.ActiveContainersReturns(2)

				fakeContainer = new(fakes.FakeContainer)

				fakeProvider.WorkersReturns([]Worker{workerA, workerB}, nil)
			})

			Context("when a worker can locate the container", func() {
				BeforeEach(func() {
					workerA.LookupReturns(fakeContainer, nil)
					workerB.LookupReturns(nil, ErrContainerNotFound)
				})

				It("returns the container", func() {
					Ω(foundContainer).Should(Equal(fakeContainer))
				})

				It("looks up by the given handle", func() {
					Ω(workerA.LookupCallCount()).Should(Equal(1))
					Ω(workerB.LookupCallCount()).Should(Equal(1))

					Ω(workerA.LookupArgsForCall(0)).Should(Equal(handle))
					Ω(workerB.LookupArgsForCall(0)).Should(Equal(handle))
				})
			})

			Context("when no workers can locate the container", func() {
				BeforeEach(func() {
					workerA.LookupReturns(nil, ErrContainerNotFound)
					workerB.LookupReturns(nil, ErrContainerNotFound)
				})

				It("returns ErrContainerNotFound", func() {
					Ω(lookupErr).Should(Equal(ErrContainerNotFound))
				})
			})
		})

		Context("with no workers", func() {
			BeforeEach(func() {
				fakeProvider.WorkersReturns([]Worker{}, nil)
			})

			It("returns ErrNoWorkers", func() {
				Ω(lookupErr).Should(Equal(ErrNoWorkers))
			})
		})

		Context("when getting the workers fails", func() {
			disaster := errors.New("nope")

			BeforeEach(func() {
				fakeProvider.WorkersReturns(nil, disaster)
			})

			It("returns the error", func() {
				Ω(lookupErr).Should(Equal(disaster))
			})
		})
	})
})
