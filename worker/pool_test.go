package worker_test

import (
	"errors"

	"github.com/cloudfoundry-incubator/garden"
	. "github.com/concourse/atc/worker"
	"github.com/concourse/atc/worker/fakes"
	"github.com/pivotal-golang/lager"
	"github.com/pivotal-golang/lager/lagertest"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Pool", func() {
	var (
		logger       *lagertest.TestLogger
		fakeProvider *fakes.FakeWorkerProvider

		pool Client
	)

	BeforeEach(func() {
		logger = lagertest.NewTestLogger("test")
		fakeProvider = new(fakes.FakeWorkerProvider)

		pool = NewPool(fakeProvider)
	})

	Describe("Create", func() {
		var (
			logger lager.Logger
			id     Identifier
			spec   ContainerSpec

			createdContainer Container
			createErr        error
		)

		BeforeEach(func() {
			logger = lagertest.NewTestLogger("test")
			id = Identifier{Name: "some-name"}
			spec = ResourceTypeContainerSpec{Type: "some-type"}
		})

		JustBeforeEach(func() {
			createdContainer, createErr = pool.CreateContainer(logger, id, spec)
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

				workerA.SatisfyingReturns(workerA, nil)
				workerB.SatisfyingReturns(workerB, nil)
				workerC.SatisfyingReturns(nil, errors.New("nope"))

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
				Ω(workerA.SatisfyingCallCount()).Should(Equal(1))
				Ω(workerA.SatisfyingArgsForCall(0)).Should(Equal(spec.WorkerSpec()))

				Ω(workerB.SatisfyingCallCount()).Should(Equal(1))
				Ω(workerB.SatisfyingArgsForCall(0)).Should(Equal(spec.WorkerSpec()))

				Ω(workerC.SatisfyingCallCount()).Should(Equal(1))
				Ω(workerC.SatisfyingArgsForCall(0)).Should(Equal(spec.WorkerSpec()))
			})

			It("creates using a random worker", func() {
				for i := 1; i < 100; i++ { // account for initial create in JustBefore
					createdContainer, createErr := pool.CreateContainer(logger, id, spec)
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
					workerA.SatisfyingReturns(nil, errors.New("nope"))
					workerB.SatisfyingReturns(nil, errors.New("nope"))
					workerC.SatisfyingReturns(nil, errors.New("nope"))
				})

				It("returns a NoCompatibleWorkersError", func() {
					Ω(createErr).Should(Equal(NoCompatibleWorkersError{
						Spec:    spec.WorkerSpec(),
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
				_, err := pool.LookupContainer(logger, handle)

				Ω(err).Should(Equal(ErrNoWorkers))
			})
		})

		Context("when getting the workers fails", func() {
			disaster := errors.New("nope")

			BeforeEach(func() {
				fakeProvider.WorkersReturns(nil, disaster)
			})

			It("returns the error", func() {
				_, err := pool.LookupContainer(logger, handle)

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
					foundContainer, err := pool.LookupContainer(logger, handle)
					Ω(err).ShouldNot(HaveOccurred())

					Ω(foundContainer).Should(Equal(fakeContainer))
				})

				It("looks up by the given identifier", func() {
					_, err := pool.LookupContainer(logger, handle)
					Ω(err).ShouldNot(HaveOccurred())

					Ω(workerA.LookupContainerCallCount()).Should(Equal(1))
					Ω(workerB.LookupContainerCallCount()).Should(Equal(1))

					_, lookedUpHandle := workerA.LookupContainerArgsForCall(0)
					Ω(lookedUpHandle).Should(Equal(handle))

					_, lookedUpHandle = workerB.LookupContainerArgsForCall(0)
					Ω(lookedUpHandle).Should(Equal(handle))
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
					workerB.LookupContainerReturns(nil, errors.New(workerBErrString))
				})

				Context("when another worker finds a container", func() {
					BeforeEach(func() {
						workerA.LookupContainerReturns(fakeContainer, nil)
					})

					It("returns the container", func() {
						foundContainer, _ := pool.LookupContainer(logger, handle)

						Ω(foundContainer).Should(Equal(fakeContainer))
					})

					It("doesn't return an error", func() {
						_, err := pool.LookupContainer(logger, handle)
						Ω(err).To(BeNil())
					})
				})

				Context("when no worker finds a container", func() {
					BeforeEach(func() {
						workerA.LookupContainerReturns(nil, garden.ContainerNotFoundError{})
					})

					It("returns an error identifing which worker errored", func() {
						_, err := pool.LookupContainer(logger, handle)
						Ω(err).NotTo(BeNil())

						mwe, ok := err.(MultiWorkerError)
						Ω(ok).To(BeTrue())
						Ω(mwe.Errors()).To(Equal(map[string]error{
							workerBName: errors.New(workerBErrString)}))
					})

				})
			})

			Context("when no workers can locate the container", func() {
				BeforeEach(func() {
					workerA.LookupContainerReturns(nil, garden.ContainerNotFoundError{})
					workerB.LookupContainerReturns(nil, garden.ContainerNotFoundError{})
				})

				It("returns ErrContainerNotFound", func() {
					_, err := pool.LookupContainer(logger, handle)
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
					_, err := pool.LookupContainer(logger, handle)

					Ω(err).Should(BeAssignableToTypeOf(MultipleWorkersFoundContainerError{}))
					Ω(err.(MultipleWorkersFoundContainerError).Names).Should(ConsistOf("worker-a", "worker-b"))
				})

				It("releases all returned containers", func() {
					_, _ = pool.LookupContainer(logger, handle)

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
					foundContainers, err := pool.FindContainersForIdentifier(logger, id)
					Ω(err).ShouldNot(HaveOccurred())

					Ω(len(foundContainers)).Should(Equal(1))
					Ω(foundContainers[0]).Should(Equal(fakeContainer))
				})

				It("looks up by the given identifier", func() {
					_, err := pool.FindContainersForIdentifier(logger, id)
					Ω(err).ShouldNot(HaveOccurred())

					Ω(workerA.FindContainersForIdentifierCallCount()).Should(Equal(1))
					Ω(workerB.FindContainersForIdentifierCallCount()).Should(Equal(1))

					_, lookedUpID := workerA.FindContainersForIdentifierArgsForCall(0)
					Ω(lookedUpID).Should(Equal(id))

					_, lookedUpID = workerB.FindContainersForIdentifierArgsForCall(0)
					Ω(lookedUpID).Should(Equal(id))
				})
			})

			Context("when no workers can locate any containers", func() {
				BeforeEach(func() {
					workerA.FindContainersForIdentifierReturns(nil, nil)
					workerB.FindContainersForIdentifierReturns(nil, nil)
				})

				It("returns empty array of containers", func() {
					foundContainers, err := pool.FindContainersForIdentifier(logger, id)
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
						foundContainers, err := pool.FindContainersForIdentifier(logger, id)
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
						foundContainers, _ := pool.FindContainersForIdentifier(logger, id)
						Ω(foundContainers).Should(ConsistOf(fakeContainer, secondFakeContainer, thirdFakeContainer))
					})

					It("returns an error identifing which worker errored", func() {
						_, err := pool.FindContainersForIdentifier(logger, id)

						Ω(err.Error()).Should(ContainSubstring(workerBName))
						Ω(err.Error()).Should(ContainSubstring(workerBErrString))

						mwe, ok := err.(MultiWorkerError)
						Ω(ok).To(BeTrue())
						Ω(mwe.Errors()).To(Equal(map[string]error{
							workerBName: errors.New(workerBErrString)}))
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
						foundContainers, _ := pool.FindContainersForIdentifier(logger, id)
						Ω(foundContainers).Should(ConsistOf(fakeContainer, secondFakeContainer, thirdFakeContainer))
					})

					It("returns an error identifing which workers errored", func() {
						_, err := pool.FindContainersForIdentifier(logger, id)

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
				_, err := pool.FindContainersForIdentifier(logger, id)

				Ω(err).Should(Equal(ErrNoWorkers))
			})
		})

		Context("when getting the workers fails", func() {
			disaster := errors.New("nope")

			BeforeEach(func() {
				fakeProvider.WorkersReturns(nil, disaster)
			})

			It("returns the error", func() {
				_, err := pool.FindContainersForIdentifier(logger, id)

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
			foundContainer, lookupErr = pool.FindContainerForIdentifier(logger, id)
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

					_, lookedUpID := workerA.FindContainerForIdentifierArgsForCall(0)
					Ω(lookedUpID).Should(Equal(id))

					_, lookedUpID = workerB.FindContainerForIdentifierArgsForCall(0)
					Ω(lookedUpID).Should(Equal(id))
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
