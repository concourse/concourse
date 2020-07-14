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
			time.Minute*10,
		)
	})

	JustBeforeEach(func() {
		err = scanner.Run(context.TODO())
	})

	Describe("Run", func() {
		var fakeLock *lockfakes.FakeLock

		BeforeEach(func() {
			fakeLock = new(lockfakes.FakeLock)
			fakeCheckFactory.AcquireScanningLockReturns(fakeLock, true, nil)
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

							Context("when try creating a check panic", func() {
								BeforeEach(func() {
									fakeCheckFactory.TryCreateCheckStub = func(context.Context, db.Checkable, db.ResourceTypes, atc.Version, bool) (db.Check, bool, error) {
										panic("something went wrong")
									}
								})

								It("recover from the panic", func() {
									Expect(err).ToNot(HaveOccurred())
									Eventually(fakeResource.SetCheckSetupErrorCallCount).Should(Equal(1))
									Eventually(fakeResource.SetCheckSetupErrorArgsForCall(0).Error).Should(ContainSubstring("something went wrong"))
								})
							})
						})

						Context("when the checkable has a pinned version", func() {
							BeforeEach(func() {
								fakeResource.CurrentPinnedVersionReturns(atc.Version{"some": "version"})
							})

							It("creates a check", func() {
								Expect(fakeCheckFactory.TryCreateCheckCallCount()).To(Equal(1))
								_, _, _, fromVersion, _ := fakeCheckFactory.TryCreateCheckArgsForCall(0)
								Expect(fromVersion).To(Equal(atc.Version{"some": "version"}))
							})

							It("clears the check error", func() {
								Expect(fakeResource.SetCheckSetupErrorCallCount()).To(Equal(1))
								Expect(fakeResource.SetCheckSetupErrorArgsForCall(0)).To(BeNil())
							})

							It("sends a notification for the checker to run", func() {
								Expect(fakeCheckFactory.NotifyCheckerCallCount()).To(Equal(1))
							})
						})

						Context("when the checkable does not have a pinned version", func() {
							BeforeEach(func() {
								fakeResource.CurrentPinnedVersionReturns(nil)
							})

							It("creates a check", func() {
								Expect(fakeCheckFactory.TryCreateCheckCallCount()).To(Equal(1))
								_, _, _, fromVersion, _ := fakeCheckFactory.TryCreateCheckArgsForCall(0)
								Expect(fromVersion).To(BeNil())
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

								_, checkable, _, _, manuallyTriggered := fakeCheckFactory.TryCreateCheckArgsForCall(0)
								Expect(checkable).To(Equal(fakeResourceType))
								Expect(manuallyTriggered).To(BeFalse())

								_, checkable, _, _, manuallyTriggered = fakeCheckFactory.TryCreateCheckArgsForCall(1)
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

		Context("when there are multiple resources that use the same resource type", func() {
			var fakeResource1, fakeResource2 *dbfakes.FakeResource
			var fakeResourceType *dbfakes.FakeResourceType

			BeforeEach(func() {
				fakeResource1 = new(dbfakes.FakeResource)
				fakeResource1.NameReturns("some-name")
				fakeResource1.SourceReturns(atc.Source{"some": "source"})
				fakeResource1.TypeReturns("custom-type")
				fakeResource1.PipelineIDReturns(1)
				fakeResource1.LastCheckEndTimeReturns(time.Now().Add(-time.Hour))

				fakeResource2 = new(dbfakes.FakeResource)
				fakeResource2.NameReturns("some-name")
				fakeResource2.SourceReturns(atc.Source{"some": "source"})
				fakeResource2.TypeReturns("custom-type")
				fakeResource2.PipelineIDReturns(1)
				fakeResource2.LastCheckEndTimeReturns(time.Now().Add(-time.Hour))

				fakeCheckFactory.ResourcesReturns([]db.Resource{fakeResource1, fakeResource2}, nil)

				fakeResourceType = new(dbfakes.FakeResourceType)
				fakeResourceType.NameReturns("custom-type")
				fakeResourceType.PipelineIDReturns(1)
				fakeResourceType.TypeReturns("some-base-type")
				fakeResourceType.SourceReturns(atc.Source{"some": "type-source"})
				fakeResourceType.LastCheckEndTimeReturns(time.Now().Add(-time.Hour))

				fakeCheckFactory.ResourceTypesReturns([]db.ResourceType{fakeResourceType}, nil)
			})

			It("only tries to create a check for the resource type once", func() {
				Expect(fakeCheckFactory.TryCreateCheckCallCount()).To(Equal(3))

				var checked []string
				_, checkable, _, _, _ := fakeCheckFactory.TryCreateCheckArgsForCall(0)
				checked = append(checked, checkable.Name())

				_, checkable, _, _, _ = fakeCheckFactory.TryCreateCheckArgsForCall(1)
				checked = append(checked, checkable.Name())

				_, checkable, _, _, _ = fakeCheckFactory.TryCreateCheckArgsForCall(2)
				checked = append(checked, checkable.Name())

				Expect(checked).To(ConsistOf([]string{fakeResourceType.Name(), fakeResource1.Name(), fakeResource2.Name()}))
			})
		})

		Context("Default with webhook check interval", func() {
			var fakeResource *dbfakes.FakeResource
			BeforeEach(func() {
				fakeResource = new(dbfakes.FakeResource)
				fakeResource.NameReturns("some-name")
				fakeResource.TagsReturns([]string{"tag-a", "tag-b"})
				fakeResource.SourceReturns(atc.Source{"some": "source"})
				fakeResource.TypeReturns("base-type")
				fakeResource.CheckEveryReturns("")
				fakeCheckFactory.ResourcesReturns([]db.Resource{fakeResource}, nil)

			})

			Context("resource has webhook", func() {
				BeforeEach(func() {
					fakeResource.HasWebhookReturns(true)
				})

				Context("last check is 9 minutes ago", func() {
					BeforeEach(func() {
						fakeResource.LastCheckEndTimeReturns(time.Now().Add(-time.Minute * 9))
					})

					It("does not create a check", func() {
						Expect(fakeCheckFactory.TryCreateCheckCallCount()).To(Equal(0))
					})
				})

				Context("last check is 11 minutes ago", func() {
					BeforeEach(func() {
						fakeResource.LastCheckEndTimeReturns(time.Now().Add(-time.Minute * 11))
					})

					It("does not create a check", func() {
						Expect(fakeCheckFactory.TryCreateCheckCallCount()).To(Equal(1))
					})
				})
			})
		})
	})
})
