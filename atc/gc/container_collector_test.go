package gc_test

import (
	"context"
	"errors"
	"time"

	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/gc"

	"code.cloudfoundry.org/lager/lagertest"
	"github.com/concourse/concourse/atc/db/dbfakes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("ContainerCollector", func() {
	var (
		fakeContainerRepository *dbfakes.FakeContainerRepository
		creatingContainer       *dbfakes.FakeCreatingContainer
		createdContainer        *dbfakes.FakeCreatedContainer
		destroyingContainer     *dbfakes.FakeDestroyingContainer

		collector GcCollector

		missingContainerGracePeriod time.Duration
		hijackContainerGracePeriod  time.Duration
	)

	BeforeEach(func() {
		fakeContainerRepository = new(dbfakes.FakeContainerRepository)
		creatingContainer = new(dbfakes.FakeCreatingContainer)
		createdContainer = new(dbfakes.FakeCreatedContainer)
		destroyingContainer = new(dbfakes.FakeDestroyingContainer)

		logger = lagertest.NewTestLogger("test")

		missingContainerGracePeriod = 1 * time.Minute
		hijackContainerGracePeriod = 1 * time.Minute

		collector = gc.NewContainerCollector(
			fakeContainerRepository,
			missingContainerGracePeriod,
			hijackContainerGracePeriod,
		)
	})

	Describe("Run", func() {
		var (
			err error
		)

		JustBeforeEach(func() {
			err = collector.Run(context.TODO())
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
							0, errors.New("You have to be able to accept failure to get better"),
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
				destroyingContainerFromCreated *dbfakes.FakeDestroyingContainer
			)

			BeforeEach(func() {
				creatingContainer.HandleReturns("some-handle-1")
				createdContainer.HandleReturns("some-handle-2")
				createdContainer.WorkerNameReturns("foo")

				destroyingContainerFromCreated = new(dbfakes.FakeDestroyingContainer)
				createdContainer.DestroyingReturns(destroyingContainerFromCreated, nil)
				destroyingContainerFromCreated.HandleReturns("some-handle-2")
				destroyingContainerFromCreated.WorkerNameReturns("foo")

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

			Context("when there are created containers that haven't been hijacked", func() {
				BeforeEach(func() {
					createdContainer.LastHijackReturns(time.Time{})
				})

				It("succeeds", func() {
					Expect(err).ToNot(HaveOccurred())
				})

				It("marks the container as destroying", func() {
					Expect(createdContainer.DestroyingCallCount()).To(Equal(1))
				})
			})

			Context("when there are created containers that were hijacked beyond the grace period", func() {
				BeforeEach(func() {
					createdContainer.LastHijackReturns(time.Now().Add(-1 * time.Hour))
				})

				It("succeeds", func() {
					Expect(err).ToNot(HaveOccurred())
				})

				It("marks the container as destroying", func() {
					Expect(createdContainer.DestroyingCallCount()).To(Equal(1))
				})
			})

			Context("when there are created containers hijacked recently", func() {

				BeforeEach(func() {
					createdContainer.LastHijackReturns(time.Now())
				})

				It("succeeds", func() {
					Expect(err).ToNot(HaveOccurred())
				})

				It("does not destroy them", func() {
					Expect(createdContainer.DestroyingCallCount()).To(Equal(0))
				})
			})

			It("marks all found containers (created and destroying only, no creating) as destroying", func() {
				Expect(fakeContainerRepository.FindOrphanedContainersCallCount()).To(Equal(1))

				Expect(createdContainer.DestroyingCallCount()).To(Equal(1))

				Expect(destroyingContainerFromCreated.DestroyCallCount()).To(Equal(0))

				Expect(destroyingContainer.DestroyCallCount()).To(Equal(0))
			})

			Context("when finding containers for deletion fails", func() {
				BeforeEach(func() {
					fakeContainerRepository.FindOrphanedContainersReturns(nil, nil, nil, errors.New("some error"))
				})

				It("returns and logs the error", func() {
					Expect(err.Error()).To(ContainSubstring("some error"))
					Expect(fakeContainerRepository.FindOrphanedContainersCallCount()).To(Equal(1))
				})
			})
		})
	})
})
