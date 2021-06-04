package db_test

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/db/dbfakes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("CheckFactory", func() {
	var (
		err     error
		created bool
		build   db.Build
	)

	Describe("TryCreateCheck", func() {
		var (
			fakeResource         *dbfakes.FakeResource
			fakeResourceType     *dbfakes.FakeResourceType
			fakeResourceTypes    db.ResourceTypes
			fakeVarSourceConfigs atc.VarSourceConfigs
			fromVersion          atc.Version
			manuallyTriggered    bool

			checkPlan atc.Plan
			fakeBuild *dbfakes.FakeBuild
		)

		BeforeEach(func() {
			fromVersion = atc.Version{"from": "version"}

			planFactory := atc.NewPlanFactory(0)
			checkPlan = planFactory.NewPlan(atc.CheckPlan{
				Type:   "doesnt-matter",
				Source: atc.Source{"doesnt": "matter"},
			})

			fakeResource = new(dbfakes.FakeResource)
			fakeResource.NameReturns("some-name")
			fakeResource.TagsReturns([]string{"tag-a", "tag-b"})
			fakeResource.SourceReturns(atc.Source{"some": "source"})
			fakeResource.PipelineIDReturns(defaultPipeline.ID())
			fakeResource.PipelineNameReturns(defaultPipeline.Name())
			fakeResource.PipelineInstanceVarsReturns(defaultPipeline.InstanceVars())
			fakeResource.PipelineReturns(defaultPipeline, true, nil)
			fakeResource.CheckPlanReturns(checkPlan, nil)

			fakeBuild = new(dbfakes.FakeBuild)
			fakeResource.CreateBuildReturns(fakeBuild, true, nil)

			fakeResourceType = new(dbfakes.FakeResourceType)
			fakeResourceType.NameReturns("some-type")
			fakeResourceType.TypeReturns("some-base-type")
			fakeResourceType.TagsReturns([]string{"some-tag"})
			fakeResourceType.SourceReturns(atc.Source{"some": "type-source"})
			fakeResourceType.DefaultsReturns(atc.Source{"some-default": "some-default-value"})
			fakeResourceType.PipelineIDReturns(defaultPipeline.ID())
			fakeResourceType.PipelineNameReturns(defaultPipeline.Name())
			fakeResourceType.PipelineInstanceVarsReturns(defaultPipeline.InstanceVars())
			fakeResourceType.PipelineReturns(defaultPipeline, true, nil)

			fakeResourceTypes = db.ResourceTypes{fakeResourceType}
			manuallyTriggered = false

			fakeVarSourceConfigs = atc.VarSourceConfigs{
				{
					Name: "some-var-source",
					Type: "vault",
					Config: map[string]interface{}{
						"some": "config",
					},
				},
				{
					Name: "other-var-source",
					Type: "dummy",
					Config: map[string]interface{}{
						"other": "config",
					},
				},
			}
		})

		Context("when it is run on a resource", func() {
			JustBeforeEach(func() {
				build, created, err = checkFactory.TryCreateCheck(context.TODO(), fakeResource, fakeResourceTypes, fakeVarSourceConfigs, fromVersion, manuallyTriggered, false)
			})

			Context("when the resource parent type is not a custom type", func() {
				BeforeEach(func() {
					fakeResource.TypeReturns("base-type")
				})

				It("returns the build", func() {
					Expect(err).NotTo(HaveOccurred())
					Expect(created).To(BeTrue())
					Expect(build).To(Equal(fakeBuild))
				})

				It("starts the build with the check plan", func() {
					Expect(fakeResource.CreateBuildCallCount()).To(Equal(1))
					_, manuallyTriggered, plan := fakeResource.CreateBuildArgsForCall(0)
					Expect(manuallyTriggered).To(BeFalse())
					Expect(plan).To(Equal(checkPlan))
				})

				Context("when the interval has not elapsed", func() {
					BeforeEach(func() {
						fakeResource.LastCheckEndTimeReturns(time.Now().Add(defaultCheckInterval))
					})

					It("does not create a build for the resource", func() {
						Expect(fakeResource.CheckPlanCallCount()).To(Equal(0))
						Expect(fakeResource.CreateBuildCallCount()).To(Equal(0))
					})

					Context("but the check is manually triggered", func() {
						BeforeEach(func() {
							manuallyTriggered = true
						})

						It("creates the build anyway", func() {
							Expect(fakeResource.CheckPlanCallCount()).To(Equal(1))
							Expect(fakeResource.CreateBuildCallCount()).To(Equal(1))
						})
					})
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
			})

			Context("when the resource has a webhook configured", func() {
				BeforeEach(func() {
					fakeResource.HasWebhookReturns(true)
				})

				It("creates a check plan with the default webhook interval", func() {
					Expect(fakeResource.CheckPlanCallCount()).To(Equal(1))
					_, types, varSources, version, interval, defaults, _, _ := fakeResource.CheckPlanArgsForCall(0)
					Expect(version).To(Equal(atc.Version{"from": "version"}))
					Expect(interval).To(Equal(defaultWebhookCheckInterval))
					Expect(types).To(HaveLen(0))
					Expect(varSources).To(Equal(fakeVarSourceConfigs))
					Expect(defaults).To(BeEmpty())
				})

				Context("when the default webhook interval has not elapsed", func() {
					BeforeEach(func() {
						fakeResource.LastCheckEndTimeReturns(time.Now().Add(-(defaultWebhookCheckInterval / 2)))
					})

					It("does not create a build for the resource", func() {
						Expect(fakeResource.CheckPlanCallCount()).To(Equal(0))
						Expect(fakeResource.CreateBuildCallCount()).To(Equal(0))
					})
				})
			})

			Context("when an interval is specified", func() {
				BeforeEach(func() {
					fakeResource.CheckEveryReturns(&atc.CheckEvery{Interval: 42 * time.Second})
				})

				It("sets it in the check plan", func() {
					Expect(fakeResource.CheckPlanCallCount()).To(Equal(1))
					_, types, varSources, version, interval, defaults, _, _ := fakeResource.CheckPlanArgsForCall(0)
					Expect(version).To(Equal(atc.Version{"from": "version"}))
					Expect(interval).To(Equal(42 * time.Second))
					Expect(types).To(HaveLen(0))
					Expect(varSources).To(Equal(fakeVarSourceConfigs))
					Expect(defaults).To(BeEmpty())
				})
			})

			Context("when CheckEvery is never", func() {
				BeforeEach(func() {
					fakeResource.CheckEveryReturns(&atc.CheckEvery{Never: true})
				})

				It("does not try parsing the interval", func() {
					Expect(err).ToNot(HaveOccurred())
				})
			})

			Context("when the resource has a parent type", func() {
				BeforeEach(func() {
					fakeResource.TypeReturns("custom-type")
					fakeResource.PipelineIDReturns(1)
					fakeResourceType.NameReturns("custom-type")
					fakeResourceType.PipelineIDReturns(1)
					fakeResourceType.DefaultsReturns(atc.Source{"sdk": "sdk"})
				})

				Context("when the checkable's interval has elapsed", func() {
					BeforeEach(func() {
						fakeResource.LastCheckEndTimeReturns(time.Now().Add(-defaultCheckInterval))
					})

					It("creates a check plan", func() {
						Expect(fakeResource.CheckPlanCallCount()).To(Equal(1))
						_, types, varSources, version, interval, defaults, _, _ := fakeResource.CheckPlanArgsForCall(0)
						Expect(version).To(Equal(atc.Version{"from": "version"}))
						Expect(interval).To(Equal(defaultCheckInterval))
						Expect(types).To(Equal(atc.ResourceTypes{
							atc.ResourceType{
								Name:   "custom-type",
								Type:   "some-base-type",
								Tags:   atc.Tags{"some-tag"},
								Source: atc.Source{"some": "type-source"},
								Defaults: atc.Source{
									"sdk": "sdk",
								},
							},
						}))
						Expect(varSources).To(Equal(fakeVarSourceConfigs))
						Expect(defaults).To(Equal(atc.Source{"sdk": "sdk"}))
					})

					It("returns the build", func() {
						Expect(err).NotTo(HaveOccurred())
						Expect(created).To(BeTrue())
						Expect(build).To(Equal(fakeBuild))
					})

					It("starts the build with the check plan", func() {
						Expect(fakeResource.CreateBuildCallCount()).To(Equal(1))
						_, manuallyTriggered, plan := fakeResource.CreateBuildArgsForCall(0)
						Expect(manuallyTriggered).To(BeFalse())
						Expect(plan.ID).ToNot(BeEmpty())
						Expect(plan.Check).To(Equal(checkPlan.Check))
					})

					Context("when creating the check plan fails", func() {
						BeforeEach(func() {
							fakeResource.CheckPlanReturns(atc.Plan{}, errors.New("failed"))
						})

						It("does not create build", func() {
							Expect(fakeResource.CreateBuildCallCount()).To(Equal(0))
							Expect(err).To(HaveOccurred())
							Expect(created).To(BeFalse())
							Expect(build).To(BeNil())
						})
					})
				})
			})
		})

		Context("when it is run for a resource type", func() {
			BeforeEach(func() {
				fakeResourceType.CheckPlanReturns(checkPlan, nil)
				fakeResourceType.CreateBuildReturns(fakeBuild, true, nil)
			})

			JustBeforeEach(func() {
				build, created, err = checkFactory.TryCreateCheck(context.TODO(), fakeResourceType, fakeResourceTypes, fakeVarSourceConfigs, fromVersion, manuallyTriggered, false)
			})

			It("creates a check plan", func() {
				Expect(fakeResourceType.CheckPlanCallCount()).To(Equal(1))
				_, types, varSources, version, interval, defaults, _, _ := fakeResourceType.CheckPlanArgsForCall(0)
				Expect(version).To(Equal(atc.Version{"from": "version"}))
				Expect(interval).To(Equal(defaultCheckInterval))
				Expect(types).To(Equal(atc.ResourceTypes{}))
				Expect(varSources).To(Equal(fakeVarSourceConfigs))
				Expect(defaults).To(Equal(atc.Source{}))
			})

			It("returns the build", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(created).To(BeTrue())
				Expect(build).To(Equal(fakeBuild))
			})

			It("starts the build with the check plan", func() {
				Expect(fakeResourceType.CreateBuildCallCount()).To(Equal(1))
				_, manuallyTriggered, plan := fakeResourceType.CreateBuildArgsForCall(0)
				Expect(manuallyTriggered).To(BeFalse())
				Expect(plan).To(Equal(checkPlan))
			})
		})
	})

	Describe("Resources", func() {
		var (
			resources       []db.Resource
			putOnlyResource db.Resource
		)

		BeforeEach(func() {
			defaultPipelineConfig = atc.Config{
				Jobs: atc.JobConfigs{
					{
						Name: "some-job",
						PlanSequence: []atc.Step{
							{
								Config: &atc.GetStep{
									Name: "some-resource",
								},
							},
							{
								Config: &atc.PutStep{
									Name: "some-put-only-resource",
								},
							},
						},
					},
				},
				Resources: atc.ResourceConfigs{
					{
						Name: "some-resource",
						Type: "some-base-resource-type",
						Source: atc.Source{
							"some": "source",
						},
					},
					{
						Name: "some-put-only-resource",
						Type: "some-base-resource-type",
						Source: atc.Source{
							"some": "source",
						},
					},
				},
				ResourceTypes: atc.ResourceTypes{
					{
						Name: "some-type",
						Type: "some-base-resource-type",
						Source: atc.Source{
							"some-type": "source",
						},
					},
				},
			}

			defaultPipelineRef = atc.PipelineRef{Name: "default-pipeline", InstanceVars: atc.InstanceVars{"branch": "master"}}
			defaultPipeline, _, err = defaultTeam.SavePipeline(defaultPipelineRef, defaultPipelineConfig, db.ConfigVersion(1), false)
			Expect(err).NotTo(HaveOccurred())

			var found bool
			putOnlyResource, found, err = defaultPipeline.Resource("some-put-only-resource")
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())
		})

		JustBeforeEach(func() {
			resources, err = checkFactory.Resources()
			Expect(err).NotTo(HaveOccurred())
		})

		It("include only resources-in-use in return", func() {
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

		Context("when a put-only resource", func() {
			for _, status := range []db.BuildStatus{db.BuildStatusAborted, db.BuildStatusErrored, db.BuildStatusFailed} {
				Context(fmt.Sprintf("has a build that %s", status), func() {
					BeforeEach(func() {
						By("creating a failed build for the put-only resource")
						build, created, err = putOnlyResource.CreateBuild(context.TODO(), false, atc.Plan{})
						Expect(err).ToNot(HaveOccurred())
						Expect(build.Finish(status)).To(Succeed())
					})
					It("returns the resource", func() {
						Expect(resources).To(HaveLen(2))
					})
				})
			}
			Context("has NOT errored", func() {
				BeforeEach(func() {
					By("creating a successful build for the put-only resource")
					build, created, err = putOnlyResource.CreateBuild(context.TODO(), false, atc.Plan{})
					Expect(err).ToNot(HaveOccurred())
					Expect(build.Finish(db.BuildStatusSucceeded)).To(Succeed())
				})
				It("returns does not return the resource", func() {
					Expect(resources).To(HaveLen(1))
				})
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

		Context("when the pipeline is paused", func() {
			BeforeEach(func() {
				_, err = dbConn.Exec(`UPDATE pipelines SET paused = true`)
				Expect(err).NotTo(HaveOccurred())
			})

			It("does not return resource types from paused pipelines", func() {
				Expect(resourceTypes).To(HaveLen(0))
			})
		})
	})

	Describe("VarSources", func() {
		var (
			varSourceConfigs     map[int]atc.VarSourceConfigs
			pipeline1, pipeline2 db.Pipeline
		)

		BeforeEach(func() {
			var err error
			pipeline1, _, err = defaultTeam.SavePipeline(atc.PipelineRef{Name: "some-pipeline"}, atc.Config{
				VarSources: atc.VarSourceConfigs{
					{
						Name: "some-var-source",
						Type: "dummy",
						Config: map[string]interface{}{
							"some": "config",
						},
					},
					{
						Name: "other-var-source",
						Type: "vault",
						Config: map[string]interface{}{
							"some": "other-config",
						},
					},
				},
			}, db.ConfigVersion(1), false)
			Expect(err).NotTo(HaveOccurred())

			pipeline2, _, err = defaultTeam.SavePipeline(atc.PipelineRef{Name: "other-pipeline"}, atc.Config{
				VarSources: atc.VarSourceConfigs{
					{
						Name: "fake-var-source",
						Type: "dummy",
						Config: map[string]interface{}{
							"fake": "config",
						},
					},
				},
			}, db.ConfigVersion(1), false)
			Expect(err).NotTo(HaveOccurred())

			_, _, err = defaultTeam.SavePipeline(atc.PipelineRef{Name: "another-pipeline"}, atc.Config{
				Jobs: atc.JobConfigs{
					{
						Name: "some-job",
					},
				},
			}, db.ConfigVersion(1), false)
			Expect(err).NotTo(HaveOccurred())
		})

		JustBeforeEach(func() {
			varSourceConfigs, err = checkFactory.VarSources()
			Expect(err).NotTo(HaveOccurred())
		})

		It("include var sources for each pipeline in return", func() {
			Expect(varSourceConfigs).To(HaveLen(2))
			Expect(varSourceConfigs).To(Equal(map[int]atc.VarSourceConfigs{
				pipeline1.ID(): atc.VarSourceConfigs{
					{
						Name: "some-var-source",
						Type: "dummy",
						Config: map[string]interface{}{
							"some": "config",
						},
					},
					{
						Name: "other-var-source",
						Type: "vault",
						Config: map[string]interface{}{
							"some": "other-config",
						},
					},
				},
				pipeline2.ID(): atc.VarSourceConfigs{
					{
						Name: "fake-var-source",
						Type: "dummy",
						Config: map[string]interface{}{
							"fake": "config",
						},
					},
				},
			}))
		})

		Context("when the pipeline is paused", func() {
			BeforeEach(func() {
				_, err = dbConn.Exec(`UPDATE pipelines SET paused = true`)
				Expect(err).NotTo(HaveOccurred())
			})

			It("does not return var source configs from paused pipelines", func() {
				Expect(varSourceConfigs).To(HaveLen(0))
			})
		})
	})
})
