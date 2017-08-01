package gc_test

import (
	"errors"
	"time"

	"github.com/concourse/atc/db"
	"github.com/concourse/atc/gc"
	"github.com/concourse/atc/gc/gcfakes"
	"github.com/concourse/atc/worker/workerfakes"

	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/garden/gardenfakes"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"
	"github.com/concourse/atc/db/dbfakes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("ContainerCollector", func() {
	var (
		fakeContainerFactory *gcfakes.FakeContainerFactory
		fakeJobRunner        *gcfakes.FakeWorkerJobRunner

		logger *lagertest.TestLogger

		creatingContainer               *dbfakes.FakeCreatingContainer
		createdContainerFromCreating    *dbfakes.FakeCreatedContainer
		destroyingContainerFromCreating *dbfakes.FakeDestroyingContainer

		createdContainer               *dbfakes.FakeCreatedContainer
		destroyingContainerFromCreated *dbfakes.FakeDestroyingContainer

		destroyingContainer *dbfakes.FakeDestroyingContainer

		fakeWorker       *workerfakes.FakeWorker
		fakeGardenClient *gardenfakes.FakeClient

		collector gc.Collector
	)

	BeforeEach(func() {
		fakeContainerFactory = new(gcfakes.FakeContainerFactory)

		fakeWorker = new(workerfakes.FakeWorker)
		fakeGardenClient = new(gardenfakes.FakeClient)
		fakeWorker.GardenClientReturns(fakeGardenClient)
		fakeJobRunner = new(gcfakes.FakeWorkerJobRunner)
		fakeJobRunner.TryStub = func(logger lager.Logger, workerName string, job gc.Job) {
			job.Run(fakeWorker)
		}

		logger = lagertest.NewTestLogger("test")

		creatingContainer = new(dbfakes.FakeCreatingContainer)
		creatingContainer.HandleReturns("some-handle-1")

		createdContainerFromCreating = new(dbfakes.FakeCreatedContainer)
		creatingContainer.CreatedReturns(createdContainerFromCreating, nil)
		createdContainerFromCreating.HandleReturns("some-handle-1")
		createdContainerFromCreating.WorkerNameReturns("foo")

		destroyingContainerFromCreating = new(dbfakes.FakeDestroyingContainer)
		createdContainerFromCreating.DestroyingReturns(destroyingContainerFromCreating, nil)
		destroyingContainerFromCreating.HandleReturns("some-handle-1")
		destroyingContainerFromCreating.WorkerNameReturns("foo")

		createdContainer = new(dbfakes.FakeCreatedContainer)
		createdContainer.HandleReturns("some-handle-2")
		createdContainer.WorkerNameReturns("foo")

		destroyingContainerFromCreated = new(dbfakes.FakeDestroyingContainer)
		createdContainer.DestroyingReturns(destroyingContainerFromCreated, nil)
		destroyingContainerFromCreated.HandleReturns("some-handle-2")
		destroyingContainerFromCreated.WorkerNameReturns("foo")

		destroyingContainer = new(dbfakes.FakeDestroyingContainer)
		destroyingContainer.HandleReturns("some-handle-3")
		destroyingContainer.WorkerNameReturns("bar")

		fakeContainerFactory.FindContainersForDeletionReturns(
			[]db.CreatingContainer{
				creatingContainer,
			},
			[]db.CreatedContainer{
				createdContainer,
			},
			[]db.DestroyingContainer{
				destroyingContainer,
			},
			nil,
		)

		destroyingContainerFromCreating.DestroyReturns(true, nil)
		destroyingContainerFromCreated.DestroyReturns(true, nil)
		destroyingContainer.DestroyReturns(true, nil)

		collector = gc.NewContainerCollector(
			logger,
			fakeContainerFactory,
			fakeJobRunner,
		)
	})

	Describe("Run", func() {
		var (
			err error
		)

		JustBeforeEach(func() {
			err = collector.Run()
		})

		It("succeeds", func() {
			Expect(err).NotTo(HaveOccurred())
		})

		Context("when there are created containers in hijacked state", func() {
			var (
				fakeGardenContainer *gardenfakes.FakeContainer
			)

			BeforeEach(func() {
				createdContainer.IsHijackedReturns(true)
				fakeGardenContainer = new(gardenfakes.FakeContainer)
			})

			Context("when container still exists in garden", func() {
				BeforeEach(func() {
					fakeGardenClient.LookupReturns(fakeGardenContainer, nil)
				})

				It("tells garden to set the TTL to 5 Min", func() {
					Expect(fakeGardenClient.LookupCallCount()).To(Equal(1))
					lookupHandle := fakeGardenClient.LookupArgsForCall(0)
					Expect(lookupHandle).To(Equal("some-handle-2"))

					Expect(fakeGardenContainer.SetGraceTimeCallCount()).To(Equal(1))
					graceTime := fakeGardenContainer.SetGraceTimeArgsForCall(0)
					Expect(graceTime).To(Equal(5 * time.Minute))
				})

				It("marks container as discontinued in database", func() {
					Expect(createdContainer.DiscontinueCallCount()).To(Equal(1))
				})
			})

			Context("when container does not exist in garden", func() {
				BeforeEach(func() {
					fakeGardenClient.LookupReturns(nil, garden.ContainerNotFoundError{Handle: "im-fake-and-still-hijacked"})
				})

				It("marks container as destroying", func() {
					Expect(createdContainer.DestroyingCallCount()).To(Equal(1))
				})
			})
		})

		It("marks all found containers as destroying, tells garden to destroy it, and then removes it from the DB", func() {
			Expect(fakeContainerFactory.FindContainersForDeletionCallCount()).To(Equal(1))

			Expect(creatingContainer.CreatedCallCount()).To(Equal(1))
			Expect(createdContainerFromCreating.DestroyingCallCount()).To(Equal(1))
			Expect(destroyingContainerFromCreating.DestroyCallCount()).To(Equal(1))

			Expect(createdContainer.DestroyingCallCount()).To(Equal(1))
			Expect(destroyingContainerFromCreated.DestroyCallCount()).To(Equal(1))

			Expect(destroyingContainer.DestroyCallCount()).To(Equal(1))

			Expect(fakeJobRunner.TryCallCount()).To(Equal(3))
			_, try1Worker, _ := fakeJobRunner.TryArgsForCall(0)
			Expect(try1Worker).To(Equal("foo"))
			_, try2Worker, _ := fakeJobRunner.TryArgsForCall(1)
			Expect(try2Worker).To(Equal("foo"))
			_, try3Worker, _ := fakeJobRunner.TryArgsForCall(2)
			Expect(try3Worker).To(Equal("bar"))

			Expect(fakeGardenClient.DestroyCallCount()).To(Equal(3))
			Expect(fakeGardenClient.DestroyArgsForCall(0)).To(Equal("some-handle-2"))
			Expect(fakeGardenClient.DestroyArgsForCall(1)).To(Equal("some-handle-1"))
			Expect(fakeGardenClient.DestroyArgsForCall(2)).To(Equal("some-handle-3"))
		})

		Context("when there are destroying containers that are discontinued", func() {
			BeforeEach(func() {
				destroyingContainer.IsDiscontinuedReturns(true)
			})

			Context("when container exists in garden", func() {
				BeforeEach(func() {
					fakeGardenClient.LookupReturns(new(gardenfakes.FakeContainer), nil)
				})

				It("does not delete container and lets it expire in garden first", func() {
					Expect(fakeGardenClient.DestroyCallCount()).To(Equal(2))
					Expect(fakeGardenClient.DestroyArgsForCall(0)).To(Equal("some-handle-2"))
					Expect(fakeGardenClient.DestroyArgsForCall(1)).To(Equal("some-handle-1"))

					Expect(destroyingContainer.DestroyCallCount()).To(Equal(0))
				})
			})

			Context("when container does not exist in garden", func() {
				BeforeEach(func() {
					fakeGardenClient.LookupReturns(nil, garden.ContainerNotFoundError{})
				})

				It("deletes container in database", func() {
					Expect(fakeGardenClient.DestroyCallCount()).To(Equal(2))
					Expect(fakeGardenClient.DestroyArgsForCall(0)).To(Equal("some-handle-2"))
					Expect(fakeGardenClient.DestroyArgsForCall(1)).To(Equal("some-handle-1"))

					Expect(destroyingContainer.DestroyCallCount()).To(Equal(1))
				})
			})
		})

		Context("when finding containers for deletion fails", func() {
			BeforeEach(func() {
				fakeContainerFactory.FindContainersForDeletionReturns(nil, nil, nil, errors.New("some-error"))
			})

			It("returns and logs the error", func() {
				Expect(err).To(MatchError("some-error"))
				Expect(fakeContainerFactory.FindContainersForDeletionCallCount()).To(Equal(1))
				Expect(fakeJobRunner.TryCallCount()).To(Equal(0))
			})
		})

		Context("when destroying a garden container errors", func() {
			BeforeEach(func() {
				fakeGardenClient.DestroyStub = func(handle string) error {
					switch handle {
					case "some-handle-1":
						return errors.New("some-error")
					case "some-handle-2":
						return nil
					case "some-handle-3":
						return nil
					default:
						return nil
					}
				}
			})

			It("continues destroying the rest of the containers", func() {
				Expect(err).NotTo(HaveOccurred())

				Expect(fakeJobRunner.TryCallCount()).To(Equal(3))
				Expect(fakeGardenClient.DestroyCallCount()).To(Equal(3))

				Expect(destroyingContainerFromCreating.DestroyCallCount()).To(Equal(0))
				Expect(destroyingContainerFromCreated.DestroyCallCount()).To(Equal(1))
				Expect(destroyingContainer.DestroyCallCount()).To(Equal(1))
			})
		})

		Context("when destroying a garden container errors because container is not found", func() {
			BeforeEach(func() {
				fakeGardenClient.DestroyStub = func(handle string) error {
					switch handle {
					case "some-handle-1":
						return garden.ContainerNotFoundError{Handle: "some-handle"}
					case "some-handle-2":
						return nil
					case "some-handle-3":
						return nil
					default:
						return nil
					}
				}
			})

			It("deletes container from database", func() {
				Expect(err).NotTo(HaveOccurred())

				Expect(fakeJobRunner.TryCallCount()).To(Equal(3))
				Expect(fakeGardenClient.DestroyCallCount()).To(Equal(3))

				Expect(destroyingContainerFromCreating.DestroyCallCount()).To(Equal(1))
				Expect(destroyingContainerFromCreated.DestroyCallCount()).To(Equal(1))
				Expect(destroyingContainer.DestroyCallCount()).To(Equal(1))
			})
		})

		Context("when destroying a container in the DB errors", func() {
			BeforeEach(func() {
				destroyingContainerFromCreating.DestroyReturns(false, errors.New("some-error"))
			})

			It("continues destroying the rest of the containers", func() {
				Expect(err).NotTo(HaveOccurred())

				Expect(fakeJobRunner.TryCallCount()).To(Equal(3))
				Expect(fakeGardenClient.DestroyCallCount()).To(Equal(3))
				Expect(destroyingContainerFromCreated.DestroyCallCount()).To(Equal(1))
				Expect(destroyingContainer.DestroyCallCount()).To(Equal(1))
			})
		})

		Context("when it can't find a container to destroy", func() {
			BeforeEach(func() {
				destroyingContainerFromCreating.DestroyReturns(false, nil)
			})

			It("continues destroying the rest of the containers", func() {
				Expect(err).NotTo(HaveOccurred())

				Expect(fakeJobRunner.TryCallCount()).To(Equal(3))
				Expect(fakeGardenClient.DestroyCallCount()).To(Equal(3))
				Expect(destroyingContainerFromCreated.DestroyCallCount()).To(Equal(1))
				Expect(destroyingContainer.DestroyCallCount()).To(Equal(1))
			})
		})
	})
})
