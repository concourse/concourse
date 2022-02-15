package lidar_test

import (
	"context"
	"errors"

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
		planFactory      atc.PlanFactory

		scanner Scanner
		batchSize int
	)

	BeforeEach(func() {
		planFactory = atc.NewPlanFactory(0)
		fakeCheckFactory = new(dbfakes.FakeCheckFactory)
		batchSize = 0
	})

	JustBeforeEach(func() {
		scanner = lidar.NewScanner(fakeCheckFactory, planFactory, batchSize)
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
					fakeCheckFactory.ResourceTypesByPipelineReturns(nil, errors.New("nope"))
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
					fakeCheckFactory.ResourceTypesByPipelineReturns(map[int]db.ResourceTypes{
						1: {fakeResourceType},
					}, nil)
				})

				It("does not check the resource", func() {
					Expect(fakeCheckFactory.TryCreateCheckCallCount()).To(Equal(0))
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

					fakeCheckFactory.ResourceTypesByPipelineReturns(map[int]db.ResourceTypes{1: {fakeResourceType}}, nil)
				})

				Context("when the resource parent type is a base type", func() {
					BeforeEach(func() {
						fakeCheckFactory.ResourceTypesByPipelineReturns(map[int]db.ResourceTypes{}, nil)
						fakeResource.TypeReturns("some-type")
					})

					It("creates a check with empty resource types list", func() {
						_, _, resourceTypes, _, _, _, toDb := fakeCheckFactory.TryCreateCheckArgsForCall(0)
						var nilResourceTypes db.ResourceTypes
						Expect(resourceTypes).To(Equal(nilResourceTypes))
						Expect(toDb).To(BeFalse())
					})

					Context("when the last check end time is past our interval", func() {
						It("creates a check", func() {
							Expect(fakeCheckFactory.TryCreateCheckCallCount()).To(Equal(1))
						})
					})

					Context("when the checkable has a pinned version", func() {
						BeforeEach(func() {
							fakeResource.CurrentPinnedVersionReturns(atc.Version{"some": "version"})
						})

						It("creates a check with that pinned version", func() {
							Expect(fakeCheckFactory.TryCreateCheckCallCount()).To(Equal(1))
							_, _, _, fromVersion, manuallyTriggered, _, toDb := fakeCheckFactory.TryCreateCheckArgsForCall(0)
							Expect(fromVersion).To(Equal(atc.Version{"some": "version"}))
							Expect(manuallyTriggered).To(BeFalse())
							Expect(toDb).To(BeFalse())
						})
					})

					Context("when the checkable does not have a pinned version", func() {
						BeforeEach(func() {
							fakeResource.CurrentPinnedVersionReturns(nil)
						})

						It("creates a check with a nil pinned version", func() {
							Expect(fakeCheckFactory.TryCreateCheckCallCount()).To(Equal(1))
							_, _, _, fromVersion, _, _, toDb := fakeCheckFactory.TryCreateCheckArgsForCall(0)
							Expect(fromVersion).To(BeNil())
							Expect(toDb).To(BeFalse())
						})
					})
				})

				Context("when there's a put-only resource", func() {
					BeforeEach(func() {
						By("checkFactory.Resources should not return any put-only resources")
						fakeResourceType.NameReturns("put-only-custom-type")
						fakeResourceType.PipelineIDReturns(1)
					})

					It("does not check the put-only resource", func() {
						Expect(fakeCheckFactory.TryCreateCheckCallCount()).To(Equal(1),
							"one check created for the unrelated fakeResource")
					})
				})
			})
		})

		Context("when batchSize is specified", func(){
			var fakeResource1, fakeResource2, fakeResource3, fakeResource4, fakeResource5 *dbfakes.FakeResource
			var scannedResources []db.Checkable

			BeforeEach(func() {
				fakeResource1 = new(dbfakes.FakeResource)
				fakeResource1.NameReturns("some-name")
				fakeResource1.SourceReturns(atc.Source{"some": "source"})

				fakeResource2 = new(dbfakes.FakeResource)
				fakeResource2.NameReturns("some-name")
				fakeResource2.SourceReturns(atc.Source{"some": "source"})

				fakeResource3 = new(dbfakes.FakeResource)
				fakeResource3.NameReturns("some-name")
				fakeResource3.SourceReturns(atc.Source{"some": "source"})

				fakeResource4 = new(dbfakes.FakeResource)
				fakeResource4.NameReturns("some-name")
				fakeResource4.SourceReturns(atc.Source{"some": "source"})

				fakeResource5 = new(dbfakes.FakeResource)
				fakeResource5.NameReturns("some-name")
				fakeResource5.SourceReturns(atc.Source{"some": "source"})

				fakeCheckFactory.ResourcesReturns([]db.Resource{fakeResource1, fakeResource2, fakeResource3, fakeResource4, fakeResource5}, nil)

				fakeCheckFactory.TryCreateCheckStub = func(ctx context.Context, checkable db.Checkable, types db.ResourceTypes, version atc.Version, b bool, b2 bool, b3 bool) (db.Build, bool, error) {
					scannedResources = append(scannedResources, checkable)
					return new(dbfakes.FakeBuild), true, nil
				}

				batchSize = 3
				scannedResources = []db.Checkable{}
			})

			It("should not fail", func(){
				Expect(err).ToNot(HaveOccurred())
			})

			It("scanned 3 resources", func(){
				Expect(len(scannedResources)).To(Equal(3))
			})
		})
	})
})
