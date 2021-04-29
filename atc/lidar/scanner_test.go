package lidar_test

import (
	"context"
	"errors"
	"time"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/db/dbfakes"
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

		scanner Scanner
	)

	BeforeEach(func() {
		fakeCheckFactory = new(dbfakes.FakeCheckFactory)

		scanner = lidar.NewScanner(fakeCheckFactory)
	})

	JustBeforeEach(func() {
		err = scanner.Run(context.TODO())
	})

	Describe("Run", func() {
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

			Context("when CheckEvery is never", func() {
				BeforeEach(func() {
					fakeResource.CheckEveryReturns(&atc.CheckEvery{Never: true})

					fakeResource.TypeReturns("parent")
					fakeResource.PipelineIDReturns(1)
					fakeResourceType := new(dbfakes.FakeResourceType)
					fakeResourceType.NameReturns("parent")
					fakeResourceType.PipelineIDReturns(1)
					fakeCheckFactory.ResourceTypesReturns([]db.ResourceType{fakeResourceType}, nil)
				})

				It("does not check the resource but still checks the parent", func() {
					Expect(fakeCheckFactory.TryCreateCheckCallCount()).To(Equal(1))
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
						fakeCheckFactory.ResourceTypesReturns([]db.ResourceType{}, nil)
						fakeResource.TypeReturns("some-type")
					})

					Context("when the last check end time is past our interval", func() {
						It("creates a check", func() {
							Expect(fakeCheckFactory.TryCreateCheckCallCount()).To(Equal(1))
						})

						Context("when try creating a check panics", func() {
							BeforeEach(func() {
								fakeCheckFactory.TryCreateCheckStub = func(context.Context, db.Checkable, db.ResourceTypes, atc.Version, bool) (db.Build, bool, error) {
									panic("something went wrong")
								}
							})

							It("recovers from the panic", func() {
								Expect(err).ToNot(HaveOccurred())
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
					})
				})

				Context("when the resource has a parent type", func() {
					BeforeEach(func() {
						fakeResource.TypeReturns("custom-type")
						fakeResource.PipelineIDReturns(1)
						fakeResourceType.NameReturns("custom-type")
						fakeResourceType.PipelineIDReturns(1)
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
				})

				Context("when a put-only resource has a parent-type", func() {
					BeforeEach(func() {
						By("checkFactory.Resources should not return any put-only resources")
						fakeResourceType.NameReturns("put-only-custom-type")
						fakeResourceType.PipelineIDReturns(1)
					})

					It("creates a check for only the parent and not the put-only resource", func() {
						Expect(fakeCheckFactory.TryCreateCheckCallCount()).To(Equal(2),
							"two checks created; one for the fakeResourceType and the second for the unrelated fakeResource")

						_, checkable, _, _, manuallyTriggered := fakeCheckFactory.TryCreateCheckArgsForCall(0)
						Expect(checkable).To(Equal(fakeResourceType))
						Expect(manuallyTriggered).To(BeFalse())
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
	})
})
