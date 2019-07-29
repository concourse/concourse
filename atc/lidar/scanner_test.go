package lidar_test

import (
	"context"
	"errors"
	"time"

	"code.cloudfoundry.org/lager/lagertest"
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/creds/credsfakes"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/db/dbfakes"
	"github.com/concourse/concourse/atc/db/lock/lockfakes"
	"github.com/concourse/concourse/atc/lidar"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

type Scanner interface {
	Run(ctx context.Context) error
}

var _ = Describe("Scanner", func() {
	var (
		err error

		fakeCheckFactory *dbfakes.FakeCheckFactory
		fakeSecrets      *credsfakes.FakeSecrets

		logger  *lagertest.TestLogger
		scanner Scanner
	)

	BeforeEach(func() {
		fakeCheckFactory = new(dbfakes.FakeCheckFactory)
		fakeSecrets = new(credsfakes.FakeSecrets)

		logger = lagertest.NewTestLogger("test")
		scanner = lidar.NewScanner(
			logger,
			fakeCheckFactory,
			fakeSecrets,
			time.Minute*1,
			time.Minute*1,
		)
	})

	JustBeforeEach(func() {
		err = scanner.Run(context.TODO())
	})

	Describe("Run", func() {
		Context("when acquiring scanning lock fails", func() {
			BeforeEach(func() {
				fakeCheckFactory.AcquireScanningLockReturns(nil, false, errors.New("nope"))
			})

			It("errors", func() {
				Expect(err).To(HaveOccurred())
			})
		})

		Context("when scanning lock is already held", func() {
			BeforeEach(func() {
				fakeCheckFactory.AcquireScanningLockReturns(nil, false, nil)
			})

			It("does not error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("does not continue", func() {
				Expect(fakeCheckFactory.ResourcesCallCount()).To(Equal(0))
			})
		})

		Context("when acquiring the lock succeeds", func() {
			var fakeLock *lockfakes.FakeLock

			BeforeEach(func() {
				fakeLock = new(lockfakes.FakeLock)
				fakeCheckFactory.AcquireScanningLockReturns(fakeLock, true, nil)
			})

			It("releases the lock", func() {
				Expect(fakeLock.ReleaseCallCount()).To(Equal(1))
			})

			Context("when fetching resources fails", func() {
				BeforeEach(func() {
					fakeCheckFactory.ResourcesReturns(nil, errors.New("nope"))
				})

				It("errors", func() {
					Expect(err).To(HaveOccurred())
				})
			})

			Context("when fetching resources succeeds", func() {
				var fakeResource *dbfakes.FakeResource

				BeforeEach(func() {
					fakeResource = new(dbfakes.FakeResource)
					fakeResource.NameReturns("some-name")
					fakeResource.TagsReturns([]string{"tag-a", "tag-b"})
					fakeResource.SourceReturns(atc.Source{"some": "source"})

					fakeCheckFactory.ResourcesReturns([]db.Resource{fakeResource}, nil)
				})

				Context("when fetching resource types fails", func() {
					BeforeEach(func() {
						fakeCheckFactory.ResourceTypesReturns(nil, errors.New("nope"))
					})

					It("errors", func() {
						Expect(err).To(HaveOccurred())
					})
				})

				Context("when fetching resources types succeeds", func() {
					var fakeResourceType *dbfakes.FakeResourceType

					BeforeEach(func() {
						fakeResourceType = new(dbfakes.FakeResourceType)
						fakeResourceType.NameReturns("some-type")
						fakeResourceType.TypeReturns("some-base-type")
						fakeResourceType.TagsReturns([]string{"some-tag"})
						fakeResourceType.SourceReturns(atc.Source{"some": "type-source"})

						fakeCheckFactory.ResourceTypesReturns([]db.ResourceType{fakeResourceType}, nil)
					})

					Context("when the resource parent type is a base type", func() {
						BeforeEach(func() {
							fakeResource.TypeReturns("base-type")
						})

						Context("when the check interval is parseable", func() {
							BeforeEach(func() {
								fakeResource.CheckEveryReturns("10s")
							})

							Context("when the last check end time is within our interval", func() {
								BeforeEach(func() {
									fakeResource.LastCheckEndTimeReturns(time.Now())
								})

								It("does not check", func() {
									Expect(fakeCheckFactory.CreateCheckCallCount()).To(Equal(0))
								})

								It("clears the check error", func() {
									Expect(fakeResource.SetCheckSetupErrorCallCount()).To(Equal(1))
									Expect(fakeResource.SetCheckSetupErrorArgsForCall(0)).To(BeNil())
								})
							})

							Context("when the last check end time is past our interval", func() {
								BeforeEach(func() {
									fakeResource.LastCheckEndTimeReturns(time.Now().Add(-time.Hour))
								})

								It("creates a check", func() {
									Expect(fakeCheckFactory.TryCreateCheckCallCount()).To(Equal(1))
								})

								It("clears the check error", func() {
									Expect(fakeResource.SetCheckSetupErrorCallCount()).To(Equal(1))
									Expect(fakeResource.SetCheckSetupErrorArgsForCall(0)).To(BeNil())
								})

								It("sends a notification for the checker to run", func() {
									Expect(fakeCheckFactory.NotifyCheckerCallCount()).To(Equal(1))
								})
							})
						})
					})

					Context("when the resource has a parent type", func() {
						BeforeEach(func() {
							fakeResource.TypeReturns("custom-type")
							fakeResource.PipelineIDReturns(1)
							fakeResourceType.NameReturns("custom-type")
							fakeResourceType.PipelineIDReturns(1)
						})

						Context("when it fails to create a check for parent resource", func() {
							BeforeEach(func() {
								fakeResourceType.CheckEveryReturns("not-a-duration")
							})

							It("sets the check error", func() {
								Expect(fakeResourceType.SetCheckSetupErrorCallCount()).To(Equal(1))
								Expect(fakeResource.SetCheckSetupErrorCallCount()).To(Equal(1))
								err := fakeResource.SetCheckSetupErrorArgsForCall(0)
								Expect(err.Error()).To(ContainSubstring("parent type 'custom-type' error:"))
							})

							It("does not create a check", func() {
								Expect(fakeCheckFactory.TryCreateCheckCallCount()).To(Equal(0))
							})
						})

						Context("when the parent type requires a check", func() {
							BeforeEach(func() {
								fakeResourceType.LastCheckEndTimeReturns(time.Now().Add(-time.Hour))
								fakeResource.LastCheckEndTimeReturns(time.Now().Add(-time.Hour))
							})

							Context("when the parent type has a version", func() {
								BeforeEach(func() {
									fakeResourceType.VersionReturns(atc.Version{"some": "version"})
								})

								It("creates a check for both the parent and the resource", func() {
									Expect(fakeCheckFactory.TryCreateCheckCallCount()).To(Equal(2))

									checkable, _, _, manuallyTriggered := fakeCheckFactory.TryCreateCheckArgsForCall(0)
									Expect(checkable).To(Equal(fakeResourceType))
									Expect(manuallyTriggered).To(BeFalse())

									checkable, _, _, manuallyTriggered = fakeCheckFactory.TryCreateCheckArgsForCall(1)
									Expect(checkable).To(Equal(fakeResource))
									Expect(manuallyTriggered).To(BeFalse())
								})

								It("sends a notification for the checker to run", func() {
									Expect(fakeCheckFactory.NotifyCheckerCallCount()).To(Equal(1))
								})
							})
						})
					})
				})
			})
		})
	})
})
