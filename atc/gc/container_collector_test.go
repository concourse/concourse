package gc_test

import (
	"context"
	"errors"
	"time"

	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/gc"
	"github.com/concourse/concourse/atc/gc/gcfakes"
	"github.com/concourse/concourse/atc/worker"
	"github.com/concourse/concourse/atc/worker/workerfakes"

	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"
	"github.com/concourse/concourse/atc/db/dbfakes"
	gardenfakes "github.com/concourse/concourse/atc/worker/gclient/gclientfakes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("ContainerCollector", func() {
	var (
		fakeContainerRepository *dbfakes.FakeContainerRepository
		fakeWorkerProvider      *workerfakes.FakeWorkerProvider
		fakeJobRunner           *gcfakes.FakeWorkerJobRunner

		fakeWorker       *workerfakes.FakeWorker
		fakeGardenClient *gardenfakes.FakeClient

		creatingContainer *dbfakes.FakeCreatingContainer

		collector                   gc.Collector
		realCollector               gc.Collector
		fakeCollector               gc.Collector
		missingContainerGracePeriod time.Duration
	)

	BeforeEach(func() {
		fakeContainerRepository = new(dbfakes.FakeContainerRepository)

		fakeWorker = new(workerfakes.FakeWorker)
		fakeGardenClient = new(gardenfakes.FakeClient)
		fakeWorker.GardenClientReturns(fakeGardenClient)

		fakeWorkerProvider = new(workerfakes.FakeWorkerProvider)

		logger = lagertest.NewTestLogger("test")

		fakeJobRunner = new(gcfakes.FakeWorkerJobRunner)
		fakeJobRunner.TryStub = func(logger lager.Logger, workerName string, job gc.Job) {
			job.Run(fakeWorker)
		}

		missingContainerGracePeriod = 1 * time.Minute

		realCollector = gc.NewContainerCollector(
			fakeContainerRepository,
			fakeJobRunner,
			missingContainerGracePeriod,
		)

		fakeCollector = gc.NewContainerCollector(
			fakeContainerRepository,
			fakeJobRunner,
			missingContainerGracePeriod,
		)
	})

	Describe("Run", func() {
		var (
			err error
		)

		JustBeforeEach(func() {
			err = collector.Run(context.TODO())
		})

		BeforeEach(func() {
			collector = fakeCollector
		})

		It("succeeds", func() {
			Expect(err).NotTo(HaveOccurred())
		})

		It("always tries to delete expired containers", func() {
			Expect(fakeContainerRepository.RemoveMissingContainersCallCount()).To(Equal(1))
			Expect(fakeContainerRepository.RemoveMissingContainersArgsForCall(0)).To(Equal(missingContainerGracePeriod))
		})

		Describe("Failed Containers", func() {
			Context("when there are failed containers", func() {
				It("tries to delete them from the database", func() {
					Expect(fakeContainerRepository.DestroyFailedContainersCallCount()).To(Equal(1))
				})

				Context("when destroying failed containers fails", func() {
					BeforeEach(func() {
						fakeContainerRepository.DestroyFailedContainersReturns(
							0,
							errors.New("You have to be able to accept failure to get better"),
						)
					})

					It("still tries to remove the orphaned containers", func() {
						Expect(fakeContainerRepository.FindOrphanedContainersCallCount()).To(Equal(1))
					})
				})
			})
		})

		Describe("Orphaned Containers", func() {

			var (
				createdContainer               *dbfakes.FakeCreatedContainer
				destroyingContainerFromCreated *dbfakes.FakeDestroyingContainer

				destroyingContainer *dbfakes.FakeDestroyingContainer
			)

			BeforeEach(func() {
				creatingContainer = new(dbfakes.FakeCreatingContainer)
				creatingContainer.HandleReturns("some-handle-1")
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

				fakeContainerRepository.FindOrphanedContainersReturns(
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
			})

			Context("when there are created containers in non-hijacked state", func() {
				BeforeEach(func() {
					collector = realCollector
					createdContainer.IsHijackedReturns(false)
				})

				Context("when the worker is running", func() {
					BeforeEach(func() {
						fakeWorkerProvider.RunningWorkersReturns([]worker.Worker{fakeWorker}, nil)
					})

					It("succeeds", func() {
						Expect(err).ToNot(HaveOccurred())
					})

					It("marks the container as destroying", func() {
						Eventually(func() int {
							return createdContainer.DestroyingCallCount()
						}).Should(Equal(1))
					})
				})

				Context("when the worker is not running", func() {
					BeforeEach(func() {
						fakeWorkerProvider.RunningWorkersReturns([]worker.Worker{}, nil)
					})

					It("succeeds", func() {
						Expect(err).ToNot(HaveOccurred())
					})

					It("marks the container as destroying", func() {
						Eventually(func() int {
							return createdContainer.DestroyingCallCount()
						}).Should(Equal(1))
					})
				})
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

			It("marks all found containers (created and destroying only, no creating) as destroying", func() {
				Expect(fakeContainerRepository.FindOrphanedContainersCallCount()).To(Equal(1))

				Eventually(func() int {
					return createdContainer.DestroyingCallCount()
				}).Should(Equal(1))

				Expect(destroyingContainerFromCreated.DestroyCallCount()).To(Equal(0))

				Expect(destroyingContainer.DestroyCallCount()).To(Equal(0))

				Expect(fakeJobRunner.TryCallCount()).To(Equal(1))
				_, try1Worker, _ := fakeJobRunner.TryArgsForCall(0)
				Expect(try1Worker).To(Equal("foo"))
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
						Expect(destroyingContainer.DestroyCallCount()).To(Equal(0))
					})
				})
			})

			Context("when finding containers for deletion fails", func() {
				BeforeEach(func() {
					fakeContainerRepository.FindOrphanedContainersReturns(nil, nil, nil, errors.New("some error"))
				})

				It("returns and logs the error", func() {
					Expect(err.Error()).To(ContainSubstring("some error"))
					Expect(fakeContainerRepository.FindOrphanedContainersCallCount()).To(Equal(1))
					Expect(fakeJobRunner.TryCallCount()).To(Equal(0))
				})
			})
		})
	})
})
