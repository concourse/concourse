package worker_test

import (
	"errors"

	"github.com/cloudfoundry-incubator/garden"
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
			handle string
		)

		Context("with no workers", func() {
			BeforeEach(func() {
				fakeProvider.WorkersReturns([]Worker{}, nil)
			})

			It("returns ErrNoWorkers", func() {
				_, err := pool.LookupContainer(handle)

				Ω(err).Should(Equal(ErrNoWorkers))
			})
		})

		Context("when getting the workers fails", func() {
			disaster := errors.New("nope")

			BeforeEach(func() {
				fakeProvider.WorkersReturns(nil, disaster)
			})

			It("returns the error", func() {
				_, err := pool.LookupContainer(handle)

				Ω(err).Should(Equal(disaster))
			})
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

				// TODO: why do we need to set these?
				workerA.ActiveContainersReturns(3)
				workerB.ActiveContainersReturns(2)

				fakeContainer = new(fakes.FakeContainer)
				fakeContainer.HandleReturns("fake-container")

				fakeProvider.WorkersReturns([]Worker{workerA, workerB}, nil)
			})

			Context("when a worker can locate the container", func() {
				BeforeEach(func() {
					workerA.LookupContainerReturns(fakeContainer, nil)
					workerB.LookupContainerReturns(nil, garden.ContainerNotFoundError{})
				})

				It("returns the container", func() {
					foundContainer, err := pool.LookupContainer(handle)
					Ω(err).ShouldNot(HaveOccurred())

					Ω(foundContainer).Should(Equal(fakeContainer))
				})

				It("looks up by the given identifier", func() {
					_, err := pool.LookupContainer(handle)
					Ω(err).ShouldNot(HaveOccurred())

					Ω(workerA.LookupContainerCallCount()).Should(Equal(1))
					Ω(workerB.LookupContainerCallCount()).Should(Equal(1))

					Ω(workerA.LookupContainerArgsForCall(0)).Should(Equal(handle))
					Ω(workerB.LookupContainerArgsForCall(0)).Should(Equal(handle))
				})
			})

			Context("when a worker fails for a reason other than it cannot find a container", func() {
				var (
					workerBName      string
					workerBErrString string
				)

				BeforeEach(func() {
					workerBName = "worker-b"
					workerBErrString = "some error from worker B"

					workerB.NameReturns(workerBName)

					workerA.LookupContainerReturns(fakeContainer, nil)
					workerB.LookupContainerReturns(nil, errors.New(workerBErrString))
				})

				It("returns the container", func() {
					foundContainer, _ := pool.LookupContainer(handle)

					Ω(foundContainer).Should(Equal(fakeContainer))
				})

				It("returns an error identifing which worker errored", func() {
					_, err := pool.LookupContainer(handle)
					Ω(err).NotTo(BeNil())

					Ω(err.Error()).Should(ContainSubstring(workerBName))
					Ω(err.Error()).Should(ContainSubstring(workerBErrString))
				})
			})

			Context("when no workers can locate the container", func() {
				BeforeEach(func() {
					workerA.LookupContainerReturns(nil, garden.ContainerNotFoundError{})
					workerB.LookupContainerReturns(nil, garden.ContainerNotFoundError{})
				})

				It("returns ErrContainerNotFound", func() {
					_, err := pool.LookupContainer(handle)
					Ω(err).Should(Equal(garden.ContainerNotFoundError{}))
				})
			})

			Context("when multiple workers can locate the container", func() {
				var secondFakeContainer *fakes.FakeContainer

				BeforeEach(func() {
					secondFakeContainer = new(fakes.FakeContainer)
					secondFakeContainer.HandleReturns("second-fake-container")

					workerA.LookupContainerReturns(fakeContainer, nil)
					workerB.LookupContainerReturns(secondFakeContainer, nil)

					workerA.NameReturns("worker-a")
					workerB.NameReturns("worker-b")
				})

				It("returns a MultipleWorkersFoundContainerError", func() {
					_, err := pool.LookupContainer(handle)

					Ω(err).Should(BeAssignableToTypeOf(MultipleWorkersFoundContainerError{}))
					Ω(err.(MultipleWorkersFoundContainerError).Names).Should(ConsistOf("worker-a", "worker-b"))
				})

				It("releases all returned containers", func() {
					_, _ = pool.LookupContainer(handle)

					Ω(fakeContainer.ReleaseCallCount()).Should(Equal(1))
					Ω(secondFakeContainer.ReleaseCallCount()).Should(Equal(1))
				})
			})
		})
	})

	Describe("FindContainersForIdentifier", func() {
		var (
			id Identifier
		)

		BeforeEach(func() {
			id = Identifier{Name: "some-name"}
		})

		Context("with multiple workers", func() {
			var (
				workerA *fakes.FakeWorker
				workerB *fakes.FakeWorker

				fakeContainer  *fakes.FakeContainer
				fakeContainers []Container
			)

			BeforeEach(func() {
				workerA = new(fakes.FakeWorker)
				workerB = new(fakes.FakeWorker)

				workerA.ActiveContainersReturns(3)
				workerB.ActiveContainersReturns(2)

				fakeContainer = new(fakes.FakeContainer)
				fakeContainer.HandleReturns("fake-container")

				fakeProvider.WorkersReturns([]Worker{workerA, workerB}, nil)

				fakeContainers = []Container{fakeContainer}
			})

			Context("when a worker can locate matching containers", func() {
				BeforeEach(func() {
					workerA.FindContainersForIdentifierReturns(fakeContainers, nil)
					workerB.FindContainersForIdentifierReturns(nil, nil)
				})

				It("returns the container", func() {
					foundContainers, err := pool.FindContainersForIdentifier(id)
					Ω(err).ShouldNot(HaveOccurred())

					Ω(len(foundContainers)).Should(Equal(1))
					Ω(foundContainers[0]).Should(Equal(fakeContainer))
				})

				It("looks up by the given identifier", func() {
					_, err := pool.FindContainersForIdentifier(id)
					Ω(err).ShouldNot(HaveOccurred())

					Ω(workerA.FindContainersForIdentifierCallCount()).Should(Equal(1))
					Ω(workerB.FindContainersForIdentifierCallCount()).Should(Equal(1))

					Ω(workerA.FindContainersForIdentifierArgsForCall(0)).Should(Equal(id))
					Ω(workerB.FindContainersForIdentifierArgsForCall(0)).Should(Equal(id))
				})
			})

			Context("when no workers can locate any containers", func() {
				BeforeEach(func() {
					workerA.FindContainersForIdentifierReturns(nil, nil)
					workerB.FindContainersForIdentifierReturns(nil, nil)
				})

				It("returns empty array of containers", func() {
					foundContainers, err := pool.FindContainersForIdentifier(id)
					Ω(err).ShouldNot(HaveOccurred())

					Ω(len(foundContainers)).Should(Equal(0))
				})
			})

			Context("when multiple workers locate containers", func() {
				var (
					secondFakeContainer  *fakes.FakeContainer
					thirdFakeContainer   *fakes.FakeContainer
					secondFakeContainers []Container
				)

				BeforeEach(func() {
					secondFakeContainer = new(fakes.FakeContainer)
					secondFakeContainer.HandleReturns("second-fake-container")

					thirdFakeContainer = new(fakes.FakeContainer)
					thirdFakeContainer.HandleReturns("third-fake-container")

					secondFakeContainers = []Container{secondFakeContainer, thirdFakeContainer}
				})

				Context("without error", func() {
					BeforeEach(func() {
						workerA.FindContainersForIdentifierReturns(fakeContainers, nil)
						workerB.FindContainersForIdentifierReturns(secondFakeContainers, nil)
					})

					It("returns all containers without error", func() {
						foundContainers, err := pool.FindContainersForIdentifier(id)
						Ω(err).ShouldNot(HaveOccurred())

						Ω(foundContainers).Should(ConsistOf(fakeContainer, secondFakeContainer, thirdFakeContainer))
					})
				})

				Context("when a worker reports an error", func() {
					var (
						workerBName      string
						workerBErrString string
					)

					BeforeEach(func() {
						workerBName = "worker b"
						workerBErrString = "some error"

						workerA.FindContainersForIdentifierReturns(fakeContainers, nil)
						workerB.FindContainersForIdentifierReturns(secondFakeContainers, errors.New(workerBErrString))

						workerB.NameReturns(workerBName)
					})

					It("returns all containers", func() {
						foundContainers, _ := pool.FindContainersForIdentifier(id)
						Ω(foundContainers).Should(ConsistOf(fakeContainer, secondFakeContainer, thirdFakeContainer))
					})

					It("returns an error identifing which worker errored", func() {
						_, err := pool.FindContainersForIdentifier(id)

						Ω(err.Error()).Should(ContainSubstring(workerBName))
						Ω(err.Error()).Should(ContainSubstring(workerBErrString))
					})
				})

				Context("when multiple workers report an error", func() {
					var (
						workerAName      string
						workerAErrString string

						workerBName      string
						workerBErrString string
					)

					BeforeEach(func() {
						workerAName = "worker a"
						workerAErrString = "some error a"

						workerBName = "worker b"
						workerBErrString = "some error b"

						workerA.NameReturns(workerAName)
						workerB.NameReturns(workerBName)

						workerA.FindContainersForIdentifierReturns(fakeContainers, errors.New(workerAErrString))
						workerB.FindContainersForIdentifierReturns(secondFakeContainers, errors.New(workerBErrString))
					})

					It("returns all containers", func() {
						foundContainers, _ := pool.FindContainersForIdentifier(id)
						Ω(foundContainers).Should(ConsistOf(fakeContainer, secondFakeContainer, thirdFakeContainer))
					})

					It("returns an error identifing which workers errored", func() {
						_, err := pool.FindContainersForIdentifier(id)

						Ω(err.Error()).Should(ContainSubstring(workerAName))
						Ω(err.Error()).Should(ContainSubstring(workerAErrString))
						Ω(err.Error()).Should(ContainSubstring(workerBName))
						Ω(err.Error()).Should(ContainSubstring(workerBErrString))
					})
				})
			})
		})

		Context("with no workers", func() {
			BeforeEach(func() {
				fakeProvider.WorkersReturns([]Worker{}, nil)
			})

			It("returns ErrNoWorkers", func() {
				_, err := pool.FindContainersForIdentifier(id)

				Ω(err).Should(Equal(ErrNoWorkers))
			})
		})

		Context("when getting the workers fails", func() {
			disaster := errors.New("nope")

			BeforeEach(func() {
				fakeProvider.WorkersReturns(nil, disaster)
			})

			It("returns the error", func() {
				_, err := pool.FindContainersForIdentifier(id)

				Ω(err).Should(Equal(disaster))
			})
		})
	})

	Describe("FindContainerForIdentifier", func() {
		var (
			id Identifier

			foundContainer Container
			lookupErr      error
		)

		BeforeEach(func() {
			id = Identifier{Name: "some-name"}
		})

		JustBeforeEach(func() {
			foundContainer, lookupErr = pool.FindContainerForIdentifier(id)
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
					workerA.FindContainerForIdentifierReturns(fakeContainer, nil)
					workerB.FindContainerForIdentifierReturns(nil, ErrContainerNotFound)
				})

				It("returns the container", func() {
					Ω(foundContainer).Should(Equal(fakeContainer))
				})

				It("looks up by the given identifier", func() {
					Ω(workerA.FindContainerForIdentifierCallCount()).Should(Equal(1))
					Ω(workerB.FindContainerForIdentifierCallCount()).Should(Equal(1))

					Ω(workerA.FindContainerForIdentifierArgsForCall(0)).Should(Equal(id))
					Ω(workerB.FindContainerForIdentifierArgsForCall(0)).Should(Equal(id))
				})
			})

			Context("when no workers can locate the container", func() {
				BeforeEach(func() {
					workerA.FindContainerForIdentifierReturns(nil, ErrContainerNotFound)
					workerB.FindContainerForIdentifierReturns(nil, ErrContainerNotFound)
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

					workerA.FindContainerForIdentifierReturns(fakeContainer, nil)
					workerB.FindContainerForIdentifierReturns(secondFakeContainer, nil)
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
					workerA.FindContainerForIdentifierReturns(fakeContainer, nil)
					workerB.FindContainerForIdentifierReturns(nil, multiErr)
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
					workerA.FindContainerForIdentifierReturns(nil, multiErrA)
					workerB.FindContainerForIdentifierReturns(nil, multiErrB)
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
	Describe("Name", func() {
		It("responds correctly", func() {
			Ω(pool.Name()).To(Equal("pool"))
		})
	})
})
