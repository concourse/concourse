package worker_test

import (
	"errors"

	"github.com/concourse/atc/db"
	. "github.com/concourse/atc/worker"
	"github.com/concourse/atc/worker/fakes"
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
		Context("when looking up the container info contains an error", func() {
			BeforeEach(func() {
				fakeProvider.GetContainerInfoReturns(db.ContainerInfo{}, false, errors.New("disaster"))
			})

			It("returns the error", func() {
				containerInfo, found, err := pool.LookupContainer(logger, "some-handle")
				Ω(err).Should(HaveOccurred())
				Ω(containerInfo).Should(BeNil())
				Ω(found).Should(BeFalse())
			})
		})

		Context("when looking up the container info does not find the container info", func() {
			BeforeEach(func() {
				fakeProvider.GetContainerInfoReturns(db.ContainerInfo{}, false, nil)
			})

			It("returns that it was not found", func() {
				containerInfo, found, err := pool.LookupContainer(logger, "some-handle")
				Ω(err).ShouldNot(HaveOccurred())
				Ω(containerInfo).Should(BeNil())
				Ω(found).Should(BeFalse())
			})
		})

		Context("when looking up the container info is successful", func() {
			var containerInfo db.ContainerInfo
			BeforeEach(func() {
				containerInfo = db.ContainerInfo{
					WorkerName: "some-worker",
					Handle:     "some-container-handle",
				}

				fakeProvider.GetContainerInfoReturns(containerInfo, true, nil)
			})

			It("calls to lookup the worker by name", func() {
				pool.LookupContainer(logger, "some-handle")

				Ω(fakeProvider.GetWorkerCallCount()).Should(Equal(1))

				workerName := fakeProvider.GetWorkerArgsForCall(0)
				Ω(workerName).Should(Equal("some-worker"))
			})

			Context("when looking up the worker returns an error", func() {
				BeforeEach(func() {
					fakeProvider.GetWorkerReturns(nil, false, errors.New("disaster"))
				})

				It("returns the error", func() {
					containerInfo, found, err := pool.LookupContainer(logger, "some-handle")
					Ω(err).Should(HaveOccurred())
					Ω(containerInfo).Should(BeNil())
					Ω(found).Should(BeFalse())
				})
			})

			Context("when we cannot find the worker from the container info", func() {
				BeforeEach(func() {
					fakeProvider.GetWorkerReturns(nil, false, nil)
				})

				It("returns ErrDBGardenMismatch", func() {
					containerInfo, found, err := pool.LookupContainer(logger, "some-handle")
					Ω(err).Should(Equal(ErrDBGardenMismatch))
					Ω(containerInfo).Should(BeNil())
					Ω(found).Should(BeFalse())
				})
			})

			Context("when looking up the worker is successful", func() {
				var fakeWorker *fakes.FakeWorker

				BeforeEach(func() {
					fakeWorker = new(fakes.FakeWorker)
					fakeProvider.GetWorkerReturns(fakeWorker, true, nil)
				})

				It("calls to lookup the container on the worker", func() {
					pool.LookupContainer(logger, "some-handle")

					Ω(fakeWorker.LookupContainerCallCount()).Should(Equal(1))

					_, handleArg := fakeWorker.LookupContainerArgsForCall(0)
					Ω(handleArg).Should(Equal("some-handle"))
				})

				Context("when looking up the container contains an error", func() {
					It("returns the error", func() {
						fakeWorker.LookupContainerReturns(nil, false, errors.New("disaster"))

						containerInfo, found, err := pool.LookupContainer(logger, "some-handle")
						Ω(err).Should(HaveOccurred())
						Ω(containerInfo).Should(BeNil())
						Ω(found).Should(BeFalse())
					})
				})

				Context("when the container cannot be found on the worker", func() {
					BeforeEach(func() {
						fakeWorker.LookupContainerReturns(nil, false, nil)
					})

					It("returns ErrDBGardenMismatch", func() {
						containerInfo, found, err := pool.LookupContainer(logger, "some-handle")
						Ω(err).Should(Equal(ErrDBGardenMismatch))
						Ω(containerInfo).Should(BeNil())
						Ω(found).Should(BeFalse())
					})
				})

				Context("when the finding the container on the worker is successful", func() {
					It("returns the container", func() {
						var fakeContainer *fakes.FakeContainer
						fakeContainer = new(fakes.FakeContainer)

						fakeWorker.LookupContainerReturns(fakeContainer, true, nil)

						foundContainer, found, err := pool.LookupContainer(logger, "some-handle")
						Ω(err).ShouldNot(HaveOccurred())
						Ω(found).Should(BeTrue())
						Ω(foundContainer).Should(Equal(fakeContainer))
					})
				})
			})
		})
	})

	Describe("FindContainerForIdentifier", func() {
		var identifier Identifier

		BeforeEach(func() {
			identifier = Identifier{
				Name:       "some-name",
				WorkerName: "some-worker",
			}
		})

		Context("when looking up the container info contains an error", func() {
			BeforeEach(func() {
				fakeProvider.FindContainerInfoForIdentifierReturns(db.ContainerInfo{}, false, errors.New("disaster"))
			})

			It("returns the error", func() {
				containerInfo, found, err := pool.FindContainerForIdentifier(logger, identifier)
				Ω(err).Should(HaveOccurred())
				Ω(containerInfo).Should(BeNil())
				Ω(found).Should(BeFalse())
			})
		})

		Context("when looking up the container info does not find the container info", func() {
			BeforeEach(func() {
				fakeProvider.FindContainerInfoForIdentifierReturns(db.ContainerInfo{}, false, nil)
			})

			It("returns that it was not found", func() {
				containerInfo, found, err := pool.FindContainerForIdentifier(logger, identifier)
				Ω(err).ShouldNot(HaveOccurred())
				Ω(containerInfo).Should(BeNil())
				Ω(found).Should(BeFalse())
			})
		})

		Context("when looking up the container info is successful", func() {
			var containerInfo db.ContainerInfo
			BeforeEach(func() {
				containerInfo = db.ContainerInfo{
					WorkerName: "some-worker",
					Handle:     "some-container-handle",
				}

				fakeProvider.FindContainerInfoForIdentifierReturns(containerInfo, true, nil)
			})

			It("calls to lookup the worker by name", func() {
				pool.FindContainerForIdentifier(logger, identifier)

				Ω(fakeProvider.GetWorkerCallCount()).Should(Equal(1))

				workerName := fakeProvider.GetWorkerArgsForCall(0)
				Ω(workerName).Should(Equal("some-worker"))
			})

			Context("when looking up the worker returns an error", func() {
				It("returns the error", func() {
					fakeProvider.GetWorkerReturns(nil, false, errors.New("disaster"))

					containerInfo, found, err := pool.FindContainerForIdentifier(logger, identifier)
					Ω(err).Should(HaveOccurred())
					Ω(containerInfo).Should(BeNil())
					Ω(found).Should(BeFalse())
				})
			})

			Context("when we cannot find the worker from the container info", func() {
				It("returns ErrDBGardenMismatch", func() {
					fakeProvider.GetWorkerReturns(nil, false, nil)

					containerInfo, found, err := pool.FindContainerForIdentifier(logger, identifier)
					Ω(err).Should(Equal(ErrDBGardenMismatch))
					Ω(containerInfo).Should(BeNil())
					Ω(found).Should(BeFalse())
				})
			})

			Context("when looking up the worker is successful", func() {
				var fakeWorker *fakes.FakeWorker

				BeforeEach(func() {
					fakeWorker = new(fakes.FakeWorker)
					fakeProvider.GetWorkerReturns(fakeWorker, true, nil)
				})

				It("calls to lookup the container on the worker", func() {
					pool.FindContainerForIdentifier(logger, identifier)

					Ω(fakeWorker.LookupContainerCallCount()).Should(Equal(1))

					_, handleArg := fakeWorker.LookupContainerArgsForCall(0)
					Ω(handleArg).Should(Equal("some-container-handle"))
				})

				Context("when looking up the container contains an error", func() {
					It("returns the error", func() {
						fakeWorker.LookupContainerReturns(nil, false, errors.New("disaster"))

						containerInfo, found, err := pool.FindContainerForIdentifier(logger, identifier)
						Ω(err).Should(HaveOccurred())
						Ω(containerInfo).Should(BeNil())
						Ω(found).Should(BeFalse())
					})
				})

				Context("when the container cannot be found on the worker", func() {
					It("returns ErrDBGardenMismatch", func() {
						fakeWorker.LookupContainerReturns(nil, false, nil)

						containerInfo, found, err := pool.FindContainerForIdentifier(logger, identifier)
						Ω(err).Should(Equal(ErrDBGardenMismatch))
						Ω(containerInfo).Should(BeNil())
						Ω(found).Should(BeFalse())
					})
				})

				Context("when the finding the container on the worker is successful", func() {
					It("returns the container", func() {
						var fakeContainer *fakes.FakeContainer
						fakeContainer = new(fakes.FakeContainer)

						fakeWorker.LookupContainerReturns(fakeContainer, true, nil)

						foundContainer, found, err := pool.FindContainerForIdentifier(logger, identifier)
						Ω(err).ShouldNot(HaveOccurred())
						Ω(found).Should(BeTrue())
						Ω(foundContainer).Should(Equal(fakeContainer))
					})
				})
			})
		})
	})
})
