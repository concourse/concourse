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
			id   Identifier
			spec ContainerSpec

			createdContainer Container
			createErr        error
		)

		BeforeEach(func() {
			id = Identifier{Name: "some-name"}
			spec = ResourceTypeContainerSpec{Type: "some-type"}
		})

		JustBeforeEach(func() {
			createdContainer, createErr = pool.CreateContainer(id, spec)
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
					createdContainer, createErr := pool.CreateContainer(id, spec)
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

	Describe("LookupContainer", func() {
		var (
			id Identifier

			foundContainer Container
			lookupErr      error
		)

		BeforeEach(func() {
			id = Identifier{Name: "some-name"}
		})

		JustBeforeEach(func() {
			foundContainer, lookupErr = pool.LookupContainer(id)
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
				fakeContainer.HandleReturns("fake-container")

				fakeProvider.WorkersReturns([]Worker{workerA, workerB}, nil)
			})

			Context("when a worker can locate the container", func() {
				BeforeEach(func() {
					workerA.LookupContainerReturns(fakeContainer, nil)
					workerB.LookupContainerReturns(nil, ErrContainerNotFound)
				})

				It("returns the container", func() {
					Ω(foundContainer).Should(Equal(fakeContainer))
				})

				It("looks up by the given identifier", func() {
					Ω(workerA.LookupContainerCallCount()).Should(Equal(1))
					Ω(workerB.LookupContainerCallCount()).Should(Equal(1))

					Ω(workerA.LookupContainerArgsForCall(0)).Should(Equal(id))
					Ω(workerB.LookupContainerArgsForCall(0)).Should(Equal(id))
				})
			})

			Context("when no workers can locate the container", func() {
				BeforeEach(func() {
					workerA.LookupContainerReturns(nil, ErrContainerNotFound)
					workerB.LookupContainerReturns(nil, ErrContainerNotFound)
				})

				It("returns ErrContainerNotFound", func() {
					Ω(lookupErr).Should(Equal(ErrContainerNotFound))
				})
			})

			Context("when multiple workers can locate the container", func() {
				var secondFakeContainer *fakes.FakeContainer

				BeforeEach(func() {
					secondFakeContainer = new(fakes.FakeContainer)
					secondFakeContainer.HandleReturns("second-fake-container")

					workerA.LookupContainerReturns(fakeContainer, nil)
					workerB.LookupContainerReturns(secondFakeContainer, nil)
				})

				It("returns a MultipleContainersError", func() {
					Ω(lookupErr).Should(BeAssignableToTypeOf(MultipleContainersError{}))
					Ω(lookupErr.(MultipleContainersError).Handles).Should(ConsistOf("fake-container", "second-fake-container"))
				})

				It("releases all returned containers", func() {
					Ω(fakeContainer.ReleaseCallCount()).Should(Equal(1))
					Ω(secondFakeContainer.ReleaseCallCount()).Should(Equal(1))
				})
			})

			Context("when a worker locates multiple containers", func() {
				var multiErr = MultipleContainersError{[]string{"a", "b"}}

				BeforeEach(func() {
					workerA.LookupContainerReturns(fakeContainer, nil)
					workerB.LookupContainerReturns(nil, multiErr)
				})

				It("returns a MultipleContainersError including the other found container", func() {
					Ω(lookupErr).Should(BeAssignableToTypeOf(MultipleContainersError{}))
					Ω(lookupErr.(MultipleContainersError).Handles).Should(ConsistOf("a", "b", "fake-container"))
				})

				It("releases all returned containers", func() {
					Ω(fakeContainer.ReleaseCallCount()).Should(Equal(1))
				})
			})

			Context("when multiple workers locate multiple containers", func() {
				var multiErrA = MultipleContainersError{[]string{"a", "b"}}
				var multiErrB = MultipleContainersError{[]string{"c", "d"}}

				BeforeEach(func() {
					workerA.LookupContainerReturns(nil, multiErrA)
					workerB.LookupContainerReturns(nil, multiErrB)
				})

				It("returns a MultipleContainersError including all found containers", func() {
					Ω(lookupErr).Should(BeAssignableToTypeOf(MultipleContainersError{}))
					Ω(lookupErr.(MultipleContainersError).Handles).Should(ConsistOf("a", "b", "c", "d"))
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
