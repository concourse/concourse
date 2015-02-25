package worker_test

import (
	"errors"

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
			spec ContainerSpec

			createdContainer Container
			createErr        error
		)

		BeforeEach(func() {
			spec = ResourceTypeContainerSpec{Type: "some-type"}
		})

		JustBeforeEach(func() {
			createdContainer, createErr = pool.CreateContainer("handle", spec)
		})

		Context("with multiple workers", func() {
			var (
				workerA *fakes.FakeWorker
				workerB *fakes.FakeWorker
				workerC *fakes.FakeWorker

				fakeContainer *fakes.FakeContainer
			)

			BeforeEach(func() {
				workerA = new(fakes.FakeWorker)
				workerB = new(fakes.FakeWorker)
				workerC = new(fakes.FakeWorker)

				workerA.ActiveContainersReturns(3)
				workerB.ActiveContainersReturns(2)

				workerA.SatisfiesReturns(true)
				workerB.SatisfiesReturns(true)

				fakeContainer = new(fakes.FakeContainer)
				workerA.CreateContainerReturns(fakeContainer, nil)
				workerB.CreateContainerReturns(fakeContainer, nil)
				workerC.CreateContainerReturns(fakeContainer, nil)

				fakeProvider.WorkersReturns([]Worker{workerA, workerB, workerC}, nil)
			})

			It("succeeds", func() {
				Ω(createErr).ShouldNot(HaveOccurred())
			})

			It("returns the created container", func() {
				Ω(createdContainer).Should(Equal(fakeContainer))
			})

			It("checks that the workers satisfy the given spec", func() {
				Ω(workerA.SatisfiesCallCount()).Should(Equal(1))
				Ω(workerA.SatisfiesArgsForCall(0)).Should(Equal(spec))

				Ω(workerB.SatisfiesCallCount()).Should(Equal(1))
				Ω(workerB.SatisfiesArgsForCall(0)).Should(Equal(spec))

				Ω(workerC.SatisfiesCallCount()).Should(Equal(1))
				Ω(workerC.SatisfiesArgsForCall(0)).Should(Equal(spec))
			})

			It("creates using a random worker", func() {
				for i := 1; i < 100; i++ { // account for initial create in JustBefore
					createdContainer, createErr := pool.CreateContainer("handle", spec)
					Ω(createErr).ShouldNot(HaveOccurred())
					Ω(createdContainer).Should(Equal(fakeContainer))
				}

				Ω(workerA.CreateContainerCallCount()).Should(BeNumerically("~", workerB.CreateContainerCallCount(), 50))
				Ω(workerC.CreateContainerCallCount()).Should(BeZero())
			})

			Context("when creating the container fails", func() {
				disaster := errors.New("nope")

				BeforeEach(func() {
					workerA.CreateContainerReturns(nil, disaster)
					workerB.CreateContainerReturns(nil, disaster)
				})

				It("returns the error", func() {
					Ω(createErr).Should(Equal(disaster))
				})
			})

			Context("when no workers satisfy the spec", func() {
				BeforeEach(func() {
					workerA.SatisfiesReturns(false)
					workerB.SatisfiesReturns(false)
					workerC.SatisfiesReturns(false)
				})

				It("returns a NoCompatibleWorkersError", func() {
					Ω(createErr).Should(Equal(NoCompatibleWorkersError{
						Spec:    spec,
						Workers: []Worker{workerA, workerB, workerC},
					}))
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
