package db_test

import (
	"context"
	"time"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/db/dbfakes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("CheckFactory", func() {
	var (
		err                 error
		resourceConfigScope db.ResourceConfigScope
	)

	BeforeEach(func() {
		setupTx, err := dbConn.Begin()
		Expect(err).ToNot(HaveOccurred())

		brt := db.BaseResourceType{
			Name: "some-base-resource-type",
		}

		_, err = brt.FindOrCreate(setupTx, false)
		Expect(err).NotTo(HaveOccurred())
		Expect(setupTx.Commit()).To(Succeed())

		resourceConfigScope, err = defaultResource.SetResourceConfig(
			atc.Source{"some": "repository"},
			atc.VersionedResourceTypes{},
		)
		Expect(err).NotTo(HaveOccurred())
	})

	Describe("TryCreateCheck", func() {

		var (
			created bool
			build   db.Build

			fakeResource      *dbfakes.FakeResource
			fakeResourceType  *dbfakes.FakeResourceType
			fakeResourceTypes db.ResourceTypes
			fromVersion       atc.Version
			manuallyTriggered bool

			checkPlan atc.CheckPlan
		)

		BeforeEach(func() {
			fromVersion = atc.Version{"from": "version"}

			checkPlan = atc.CheckPlan{
				Type:   "doesnt-matter",
				Source: atc.Source{"doesnt": "matter"},
			}

			fakeResource = new(dbfakes.FakeResource)
			fakeResource.NameReturns("some-name")
			fakeResource.TagsReturns([]string{"tag-a", "tag-b"})
			fakeResource.SourceReturns(atc.Source{"some": "source"})
			fakeResource.PipelineIDReturns(defaultPipeline.ID())
			fakeResource.PipelineNameReturns(defaultPipeline.Name())
			fakeResource.PipelineReturns(defaultPipeline, true, nil)
			fakeResource.CheckPlanReturns(checkPlan)

			fakeResourceType = new(dbfakes.FakeResourceType)
			fakeResourceType.NameReturns("some-type")
			fakeResourceType.TypeReturns("some-base-type")
			fakeResourceType.TagsReturns([]string{"some-tag"})
			fakeResourceType.SourceReturns(atc.Source{"some": "type-source"})
			fakeResourceType.PipelineIDReturns(defaultPipeline.ID())
			fakeResourceType.PipelineNameReturns(defaultPipeline.Name())
			fakeResourceType.PipelineReturns(defaultPipeline, true, nil)

			fakeResourceTypes = db.ResourceTypes{fakeResourceType}
			manuallyTriggered = true
		})

		JustBeforeEach(func() {
			build, created, err = checkFactory.TryCreateCheck(context.TODO(), fakeResource, fakeResourceTypes, fromVersion, manuallyTriggered)
		})

		Context("when the resource parent type is not a custom type", func() {
			BeforeEach(func() {
				fakeResource.TypeReturns("base-type")
			})

			Context("when the configured timeout is not parseable", func() {
				BeforeEach(func() {
					fakeResource.CheckTimeoutReturns("not-a-duration")
				})

				It("errors", func() {
					Expect(err).To(HaveOccurred())
				})
			})

			Context("when the configured timeout is parseable", func() {
				BeforeEach(func() {
					fakeResource.CheckTimeoutReturns("10s")
				})

				It("creates a check plan", func() {
					Expect(fakeResource.CheckPlanCallCount()).To(Equal(1))
					version, timeout, types := fakeResource.CheckPlanArgsForCall(0)
					Expect(version).To(Equal(atc.Version{"from": "version"}))
					Expect(timeout).To(Equal(10 * time.Second))
					Expect(types).To(BeNil())
				})

				Context("when a build is not created", func() {
					BeforeEach(func() {
						fakeResource.CreateBuildReturns(nil, false, nil)
					})

					It("returns false", func() {
						Expect(err).NotTo(HaveOccurred())
						Expect(created).To(BeFalse())
						Expect(build).To(BeNil())
					})
				})

				Context("when a build is created", func() {
					var fakeBuild *dbfakes.FakeBuild

					BeforeEach(func() {
						fakeBuild = new(dbfakes.FakeBuild)
						fakeResource.CreateBuildReturns(fakeBuild, true, nil)
					})

					It("returns the build", func() {
						Expect(err).NotTo(HaveOccurred())
						Expect(created).To(BeTrue())
						Expect(build).To(Equal(fakeBuild))
					})

					It("starts the build with the check plan", func() {
						Expect(fakeBuild.StartCallCount()).To(Equal(1))
						plan := fakeBuild.StartArgsForCall(0)
						Expect(plan.ID).ToNot(BeEmpty())
						Expect(plan.Check).To(Equal(&checkPlan))
					})

					Context("after starting", func() {
						BeforeEach(func() {
							fakeBuild.ReloadStub = func() (bool, error) {
								fakeBuild.StatusReturns(db.BuildStatusStarted)
								return true, nil
							}
						})

						It("reloads the build so that it returns a started build", func() {
							Expect(build.Status()).To(Equal(db.BuildStatusStarted))
						})
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

			Context("when the resource and type are properly configured", func() {
				BeforeEach(func() {
					fakeResourceType.LastCheckEndTimeReturns(time.Now().Add(-time.Hour))
					fakeResource.LastCheckEndTimeReturns(time.Now().Add(-time.Hour))

					fakeResource.SetResourceConfigReturns(resourceConfigScope, nil)
				})

				Context("when the parent type has no version", func() {
					BeforeEach(func() {
						fakeResourceType.VersionReturns(nil)
					})

					It("errors", func() {
						Expect(err).To(HaveOccurred())
					})
				})

				Context("when the parent type has a version", func() {
					BeforeEach(func() {
						fakeResourceType.VersionReturns(atc.Version{"some": "version"})
					})

					It("creates a check plan", func() {
						Expect(fakeResource.CheckPlanCallCount()).To(Equal(1))
						version, timeout, types := fakeResource.CheckPlanArgsForCall(0)
						Expect(version).To(Equal(atc.Version{"from": "version"}))
						Expect(timeout).To(Equal(defaultCheckTimeout))
						Expect(types).To(Equal(fakeResourceTypes))
					})

					Context("when a build is created", func() {
						var fakeBuild *dbfakes.FakeBuild

						BeforeEach(func() {
							fakeBuild = new(dbfakes.FakeBuild)
							fakeResource.CreateBuildReturns(fakeBuild, true, nil)
						})

						It("returns the build", func() {
							Expect(err).NotTo(HaveOccurred())
							Expect(created).To(BeTrue())
							Expect(build).To(Equal(fakeBuild))
						})

						It("starts the build with the check plan", func() {
							Expect(fakeBuild.StartCallCount()).To(Equal(1))
							plan := fakeBuild.StartArgsForCall(0)
							Expect(plan.ID).ToNot(BeEmpty())
							Expect(plan.Check).To(Equal(&checkPlan))
						})

						Context("after starting", func() {
							BeforeEach(func() {
								fakeBuild.ReloadStub = func() (bool, error) {
									fakeBuild.StatusReturns(db.BuildStatusStarted)
									return true, nil
								}
							})

							It("reloads the build so that it returns a started build", func() {
								Expect(build.Status()).To(Equal(db.BuildStatusStarted))
							})
						})
					})
				})
			})
		})
	})

	Describe("Resources", func() {
		var (
			resources []db.Resource
		)

		JustBeforeEach(func() {
			resources, err = checkFactory.Resources()
			Expect(err).NotTo(HaveOccurred())
		})

		It("include resources in return", func() {
			Expect(resources).To(HaveLen(1))
			Expect(resources[0].Name()).To(Equal("some-resource"))
		})

		Context("when the resource is not active", func() {
			BeforeEach(func() {
				_, err = dbConn.Exec(`UPDATE resources SET active = false`)
				Expect(err).NotTo(HaveOccurred())
			})

			It("does not return the resource", func() {
				Expect(resources).To(HaveLen(0))
			})
		})

		Context("when the resource pipeline is paused", func() {
			BeforeEach(func() {
				_, err = dbConn.Exec(`UPDATE pipelines SET paused = true`)
				Expect(err).NotTo(HaveOccurred())
			})

			It("does not return the resource", func() {
				Expect(resources).To(HaveLen(0))
			})
		})
	})

	Describe("ResourceTypes", func() {
		var (
			resourceTypes db.ResourceTypes
		)

		JustBeforeEach(func() {
			resourceTypes, err = checkFactory.ResourceTypes()
			Expect(err).NotTo(HaveOccurred())
		})

		It("include resource types in return", func() {
			Expect(resourceTypes).To(HaveLen(1))
			Expect(resourceTypes[0].Name()).To(Equal("some-type"))
		})

		Context("when the resource type is not active", func() {
			BeforeEach(func() {
				_, err = dbConn.Exec(`UPDATE resource_types SET active = false`)
				Expect(err).NotTo(HaveOccurred())
			})

			It("does not return the resource type", func() {
				Expect(resourceTypes).To(HaveLen(0))
			})
		})
	})
})
