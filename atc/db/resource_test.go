package db_test

import (
	"context"
	"strconv"
	"time"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/tracing"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"go.opentelemetry.io/otel/api/trace"
	"go.opentelemetry.io/otel/api/trace/tracetest"
)

var _ = Describe("Resource", func() {
	var pipeline db.Pipeline

	BeforeEach(func() {
		var (
			created bool
			err     error
		)

		pipeline, created, err = defaultTeam.SavePipeline(
			atc.PipelineRef{Name: "pipeline-with-resources"},
			atc.Config{
				Resources: atc.ResourceConfigs{
					{
						Name:         "some-resource",
						Type:         "registry-image",
						WebhookToken: "some-token",
						Source:       atc.Source{"some": "repository"},
						Version:      atc.Version{"ref": "abcdef"},
					},
					{
						Name:   "some-other-resource",
						Public: true,
						Type:   "git",
						Source: atc.Source{"some": "other-repository"},
					},
					{
						Name:   "some-secret-resource",
						Public: false,
						Type:   "git",
						Source: atc.Source{"some": "((secret-repository))"},
					},
					{
						Name:         "some-resource-custom-check",
						Type:         "git",
						Source:       atc.Source{"some": "some-repository"},
						CheckEvery:   "10ms",
						CheckTimeout: "1m",
					},
				},
				Jobs: atc.JobConfigs{
					{
						Name: "job-using-resource",
						PlanSequence: []atc.Step{
							{
								Config: &atc.GetStep{
									Name: "some-other-resource",
								},
							},
						},
					},
					{
						Name: "not-using-resource",
					},
				},
			},
			0,
			false,
		)
		Expect(err).ToNot(HaveOccurred())
		Expect(created).To(BeTrue())
	})

	Describe("(Pipeline).Resources", func() {
		var resources []db.Resource

		JustBeforeEach(func() {
			var err error
			resources, err = pipeline.Resources()
			Expect(err).ToNot(HaveOccurred())
		})

		It("returns the resources", func() {
			Expect(resources).To(HaveLen(4))

			ids := map[int]struct{}{}

			for _, r := range resources {
				ids[r.ID()] = struct{}{}

				switch r.Name() {
				case "some-resource":
					Expect(r.Type()).To(Equal("registry-image"))
					Expect(r.Source()).To(Equal(atc.Source{"some": "repository"}))
					Expect(r.ConfigPinnedVersion()).To(Equal(atc.Version{"ref": "abcdef"}))
					Expect(r.CurrentPinnedVersion()).To(Equal(r.ConfigPinnedVersion()))
					Expect(r.HasWebhook()).To(BeTrue())
				case "some-other-resource":
					Expect(r.Type()).To(Equal("git"))
					Expect(r.Source()).To(Equal(atc.Source{"some": "other-repository"}))
					Expect(r.HasWebhook()).To(BeFalse())
				case "some-secret-resource":
					Expect(r.Type()).To(Equal("git"))
					Expect(r.Source()).To(Equal(atc.Source{"some": "((secret-repository))"}))
					Expect(r.HasWebhook()).To(BeFalse())
				case "some-resource-custom-check":
					Expect(r.Type()).To(Equal("git"))
					Expect(r.Source()).To(Equal(atc.Source{"some": "some-repository"}))
					Expect(r.CheckEvery()).To(Equal("10ms"))
					Expect(r.CheckTimeout()).To(Equal("1m"))
					Expect(r.HasWebhook()).To(BeFalse())
				}
			}
		})
	})

	Describe("(Pipeline).Resource", func() {
		var (
			err      error
			found    bool
			resource db.Resource
		)

		Context("when the resource exists", func() {
			BeforeEach(func() {
				resource, found, err = pipeline.Resource("some-resource")
				Expect(err).ToNot(HaveOccurred())
			})

			It("returns the resource", func() {
				Expect(found).To(BeTrue())
				Expect(resource.Name()).To(Equal("some-resource"))
				Expect(resource.Type()).To(Equal("registry-image"))
				Expect(resource.Source()).To(Equal(atc.Source{"some": "repository"}))
			})

			Context("when the resource config id is set on the resource for the first time", func() {
				var resourceScope db.ResourceConfigScope

				BeforeEach(func() {
					setupTx, err := dbConn.Begin()
					Expect(err).ToNot(HaveOccurred())

					brt := db.BaseResourceType{
						Name: "registry-image",
					}

					_, err = brt.FindOrCreate(setupTx, false)
					Expect(err).NotTo(HaveOccurred())
					Expect(setupTx.Commit()).To(Succeed())

					resourceScope, err = resource.SetResourceConfig(atc.Source{"some": "repository"}, atc.VersionedResourceTypes{})
					Expect(err).NotTo(HaveOccurred())

					found, err = resource.Reload()
					Expect(err).NotTo(HaveOccurred())
				})

				It("returns the resource configd", func() {
					Expect(found).To(BeTrue())
					Expect(resource.ResourceConfigID()).To(Equal(resourceScope.ResourceConfig().ID()))
				})
			})

			Context("when the resource config id is not set on the resource", func() {
				It("returns no resource config id", func() {
					Expect(found).To(BeTrue())
					Expect(resource.ResourceConfigID()).To(Equal(0))
				})
			})
		})

		Context("when the resource does not exist", func() {
			JustBeforeEach(func() {
				resource, found, err = pipeline.Resource("bonkers")
				Expect(err).ToNot(HaveOccurred())
			})

			It("returns nil", func() {
				Expect(found).To(BeFalse())
				Expect(resource).To(BeNil())
			})
		})
	})

	Describe("Enable/Disable Version", func() {
		var resource db.Resource
		var rcv db.ResourceConfigVersion
		var err error
		var found bool

		BeforeEach(func() {
			resource, found, err = pipeline.Resource("some-other-resource")
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())

			setupTx, err := dbConn.Begin()
			Expect(err).ToNot(HaveOccurred())

			brt := db.BaseResourceType{
				Name: "git",
			}

			_, err = brt.FindOrCreate(setupTx, false)
			Expect(err).NotTo(HaveOccurred())
			Expect(setupTx.Commit()).To(Succeed())

			resourceScope, err := resource.SetResourceConfig(atc.Source{"some": "other-repository"}, atc.VersionedResourceTypes{})
			Expect(err).NotTo(HaveOccurred())

			err = resourceScope.SaveVersions(nil, []atc.Version{
				{"disabled": "version"},
			})
			Expect(err).ToNot(HaveOccurred())

			rcv, found, err = resourceScope.FindVersion(atc.Version{"disabled": "version"})
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())
		})

		Context("when disabling a version that exists", func() {
			var disableErr error
			var requestedSchedule1 time.Time
			var requestedSchedule2 time.Time
			var jobUsingResource db.Job
			var jobNotUsingResource db.Job

			BeforeEach(func() {
				jobUsingResource, found, err = pipeline.Job("job-using-resource")
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeTrue())

				requestedSchedule1 = jobUsingResource.ScheduleRequestedTime()

				jobNotUsingResource, found, err = pipeline.Job("not-using-resource")
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeTrue())

				requestedSchedule2 = jobNotUsingResource.ScheduleRequestedTime()

				disableErr = resource.DisableVersion(rcv.ID())
			})

			It("successfully disables the version", func() {
				Expect(disableErr).ToNot(HaveOccurred())

				versions, _, found, err := resource.Versions(db.Page{Limit: 3}, nil)
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeTrue())
				Expect(versions).To(HaveLen(1))
				Expect(versions[0].Version).To(Equal(atc.Version{"disabled": "version"}))
				Expect(versions[0].Enabled).To(BeFalse())
			})

			It("requests schedule on the jobs using that resource", func() {
				found, err := jobUsingResource.Reload()
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeTrue())

				Expect(jobUsingResource.ScheduleRequestedTime()).Should(BeTemporally(">", requestedSchedule1))
			})

			It("does not request schedule on jobs that do not use the resource", func() {
				found, err := jobNotUsingResource.Reload()
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeTrue())

				Expect(jobNotUsingResource.ScheduleRequestedTime()).Should(BeTemporally("==", requestedSchedule2))
			})

			Context("when enabling that version", func() {
				var enableErr error
				BeforeEach(func() {
					enableErr = resource.EnableVersion(rcv.ID())
				})

				It("successfully enables the version", func() {
					Expect(enableErr).ToNot(HaveOccurred())

					versions, _, found, err := resource.Versions(db.Page{Limit: 3}, nil)
					Expect(err).ToNot(HaveOccurred())
					Expect(found).To(BeTrue())
					Expect(versions).To(HaveLen(1))
					Expect(versions[0].Version).To(Equal(atc.Version{"disabled": "version"}))
					Expect(versions[0].Enabled).To(BeTrue())
				})

				It("request schedule on the jobs using that resource", func() {
					found, err := jobUsingResource.Reload()
					Expect(err).NotTo(HaveOccurred())
					Expect(found).To(BeTrue())

					Expect(jobUsingResource.ScheduleRequestedTime()).Should(BeTemporally(">", requestedSchedule1))
				})

				It("does not request schedule on jobs that do not use the resource", func() {
					found, err := jobNotUsingResource.Reload()
					Expect(err).NotTo(HaveOccurred())
					Expect(found).To(BeTrue())

					Expect(jobNotUsingResource.ScheduleRequestedTime()).Should(BeTemporally("==", requestedSchedule2))
				})
			})
		})

		Context("when disabling version that does not exist", func() {
			var disableErr error
			BeforeEach(func() {
				resource, found, err := pipeline.Resource("some-resource")
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeTrue())

				disableErr = resource.DisableVersion(123456)
			})

			It("returns an error", func() {
				Expect(disableErr).To(HaveOccurred())
			})
		})

		Context("when enabling a version that is already enabled", func() {
			var enableError error
			BeforeEach(func() {
				enableError = resource.EnableVersion(rcv.ID())
			})

			It("returns a non one row affected error", func() {
				Expect(enableError).To(Equal(db.NonOneRowAffectedError{0}))
			})
		})
	})

	Describe("SetResourceConfigScope", func() {
		var pipeline db.Pipeline
		var resource db.Resource
		var scope db.ResourceConfigScope

		BeforeEach(func() {
			config := atc.Config{
				Resources: atc.ResourceConfigs{
					{
						Name:   "some-resource",
						Type:   defaultWorkerResourceType.Type,
						Source: atc.Source{"some": "repository"},
					},
					{
						Name:   "some-other-resource",
						Type:   defaultWorkerResourceType.Type,
						Source: atc.Source{"some": "other-repository"},
					},
				},
				Jobs: atc.JobConfigs{
					{
						Name: "using-resource",
						PlanSequence: []atc.Step{
							{
								Config: &atc.GetStep{
									Name: "some-resource",
								},
							},
						},
					},
					{
						Name: "not-using-resource",
						PlanSequence: []atc.Step{
							{
								Config: &atc.GetStep{
									Name: "some-other-resource",
								},
							},
						},
					},
				},
			}

			var err error

			var created bool
			pipeline, created, err = defaultTeam.SavePipeline(
				atc.PipelineRef{Name: "some-pipeline-with-two-jobs"},
				config,
				0,
				false,
			)
			Expect(err).ToNot(HaveOccurred())
			Expect(created).To(BeTrue())

			var found bool
			resource, found, err = pipeline.Resource("some-resource")
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())

			resourceConfig, err := resourceConfigFactory.FindOrCreateResourceConfig(resource.Type(), resource.Source(), atc.VersionedResourceTypes{})
			Expect(err).ToNot(HaveOccurred())

			scope, err = resourceConfig.FindOrCreateScope(resource)
			Expect(err).ToNot(HaveOccurred())
		})

		It("associates the resource to the config and scope", func() {
			Expect(resource.ResourceConfigID()).To(BeZero())
			Expect(resource.ResourceConfigScopeID()).To(BeZero())

			Expect(resource.SetResourceConfigScope(scope)).To(Succeed())

			_, err := resource.Reload()
			Expect(err).ToNot(HaveOccurred())

			Expect(resource.ResourceConfigID()).To(Equal(scope.ResourceConfig().ID()))
			Expect(resource.ResourceConfigScopeID()).To(Equal(scope.ID()))
		})

		It("requests scheduling for downstream jobs", func() {
			job, found, err := pipeline.Job("using-resource")
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())

			otherJob, found, err := pipeline.Job("not-using-resource")
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())

			requestedSchedule := job.ScheduleRequestedTime()
			otherRequestedSchedule := otherJob.ScheduleRequestedTime()

			Expect(resource.SetResourceConfigScope(scope)).To(Succeed())

			found, err = job.Reload()
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())

			found, err = otherJob.Reload()
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())

			Expect(job.ScheduleRequestedTime()).Should(BeTemporally(">", requestedSchedule))
			Expect(otherJob.ScheduleRequestedTime()).Should(Equal(otherRequestedSchedule))
		})
	})

	Describe("CheckPlan", func() {
		var resource db.Resource
		var resourceTypes db.ResourceTypes

		BeforeEach(func() {
			var err error
			var found bool
			resource, found, err = pipeline.Resource("some-resource")
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())

			resourceTypes, err = pipeline.ResourceTypes()
			Expect(err).ToNot(HaveOccurred())
		})

		It("returns a plan which will update the resource", func() {
			defaults := atc.Source{"sdk": "sdv"}
			Expect(resource.CheckPlan(atc.Version{"some": "version"}, time.Minute, 10*time.Second, resourceTypes, defaults)).To(Equal(atc.CheckPlan{
				Name:   resource.Name(),
				Type:   resource.Type(),
				Source: defaults.Merge(resource.Source()),
				Tags:   resource.Tags(),

				FromVersion: atc.Version{"some": "version"},

				Interval: "1m0s",
				Timeout:  "10s",

				VersionedResourceTypes: resourceTypes.Deserialize(),

				Resource: resource.Name(),
			}))
		})
	})

	Describe("CreateBuild", func() {
		var ctx context.Context
		var manuallyTriggered bool
		var build db.Build
		var created bool

		BeforeEach(func() {
			ctx = context.TODO()
			manuallyTriggered = false
		})

		JustBeforeEach(func() {
			var err error
			build, created, err = defaultResource.CreateBuild(ctx, manuallyTriggered)
			Expect(err).ToNot(HaveOccurred())
		})

		It("creates a build", func() {
			Expect(created).To(BeTrue())
			Expect(build).ToNot(BeNil())
			Expect(build.PipelineID()).To(Equal(defaultResource.PipelineID()))
			Expect(build.TeamID()).To(Equal(defaultResource.TeamID()))
			Expect(build.IsManuallyTriggered()).To(BeFalse())
		})

		It("associates the resource to the build", func() {
			started, err := build.Start(atc.Plan{})
			Expect(err).ToNot(HaveOccurred())
			Expect(started).To(BeTrue())

			exists, err := build.Reload()
			Expect(err).ToNot(HaveOccurred())
			Expect(exists).To(BeTrue())

			exists, err = defaultResource.Reload()
			Expect(err).ToNot(HaveOccurred())
			Expect(exists).To(BeTrue())

			Expect(defaultResource.BuildSummary()).To(Equal(&atc.BuildSummary{
				ID:                   build.ID(),
				Name:                 strconv.Itoa(build.ID()),
				Status:               atc.StatusStarted,
				StartTime:            build.StartTime().Unix(),
				TeamName:             defaultTeam.Name(),
				PipelineID:           defaultPipeline.ID(),
				PipelineName:         defaultPipeline.Name(),
				PipelineInstanceVars: defaultPipeline.InstanceVars(),
			}))
		})

		Context("when tracing is configured", func() {
			var span trace.Span

			BeforeEach(func() {
				tracing.ConfigureTraceProvider(tracetest.NewProvider())

				ctx, span = tracing.StartSpan(context.Background(), "fake-operation", nil)
			})

			AfterEach(func() {
				tracing.Configured = false
			})

			It("propagates span context", func() {
				traceID := span.SpanContext().TraceID.String()
				buildContext := build.SpanContext()
				traceParent := buildContext.Get("traceparent")
				Expect(traceParent).To(ContainSubstring(traceID))
			})
		})

		Context("when manually triggered", func() {
			BeforeEach(func() {
				manuallyTriggered = true
			})

			It("creates a manually triggered one-off build", func() {
				Expect(build.IsManuallyTriggered()).To(BeTrue())
			})

			It("associates the resource to the build", func() {
				started, err := build.Start(atc.Plan{})
				Expect(err).ToNot(HaveOccurred())
				Expect(started).To(BeTrue())

				exists, err := build.Reload()
				Expect(err).ToNot(HaveOccurred())
				Expect(exists).To(BeTrue())

				exists, err = defaultResource.Reload()
				Expect(err).ToNot(HaveOccurred())
				Expect(exists).To(BeTrue())

				Expect(defaultResource.BuildSummary()).To(Equal(&atc.BuildSummary{
					ID:                   build.ID(),
					Name:                 strconv.Itoa(build.ID()),
					Status:               atc.StatusStarted,
					StartTime:            build.StartTime().Unix(),
					TeamName:             defaultTeam.Name(),
					PipelineID:           defaultPipeline.ID(),
					PipelineName:         defaultPipeline.Name(),
					PipelineInstanceVars: defaultPipeline.InstanceVars(),
				}))
			})

			It("can create another build", func() {
				anotherBuild, created, err := defaultResource.CreateBuild(ctx, manuallyTriggered)
				Expect(err).ToNot(HaveOccurred())
				Expect(created).To(BeTrue())
				Expect(anotherBuild.ID()).ToNot(Equal(build.ID()))
			})
		})

		Context("when not manually triggered", func() {
			BeforeEach(func() {
				manuallyTriggered = false
			})

			It("cannot create another build", func() {
				anotherBuild, created, err := defaultResource.CreateBuild(ctx, manuallyTriggered)
				Expect(err).ToNot(HaveOccurred())
				Expect(created).To(BeFalse())
				Expect(anotherBuild).To(BeNil())
			})

			It("can create a manually triggered build", func() {
				anotherBuild, created, err := defaultResource.CreateBuild(ctx, true)
				Expect(err).ToNot(HaveOccurred())
				Expect(created).To(BeTrue())
				Expect(anotherBuild.ID()).ToNot(Equal(build.ID()))

				started, err := anotherBuild.Start(atc.Plan{})
				Expect(err).ToNot(HaveOccurred())
				Expect(started).To(BeTrue())

				exists, err := anotherBuild.Reload()
				Expect(err).ToNot(HaveOccurred())
				Expect(exists).To(BeTrue())

				exists, err = defaultResource.Reload()
				Expect(err).ToNot(HaveOccurred())
				Expect(exists).To(BeTrue())

				Expect(defaultResource.BuildSummary()).To(Equal(&atc.BuildSummary{
					ID:                   anotherBuild.ID(),
					Name:                 strconv.Itoa(anotherBuild.ID()),
					Status:               atc.StatusStarted,
					StartTime:            anotherBuild.StartTime().Unix(),
					TeamName:             defaultTeam.Name(),
					PipelineID:           defaultPipeline.ID(),
					PipelineName:         defaultPipeline.Name(),
					PipelineInstanceVars: defaultPipeline.InstanceVars(),
				}))
			})

			Describe("after the first build completes", func() {
				It("can create another build after deleting the completed build", func() {
					Expect(build.Finish(db.BuildStatusSucceeded)).To(Succeed())

					anotherBuild, created, err := defaultResource.CreateBuild(ctx, manuallyTriggered)
					Expect(err).ToNot(HaveOccurred())
					Expect(created).To(BeTrue())
					Expect(anotherBuild.ID()).ToNot(Equal(build.ID()))

					found, err := build.Reload()
					Expect(err).ToNot(HaveOccurred())
					Expect(found).To(BeFalse())
				})
			})
		})
	})

	Describe("SetResourceConfig", func() {
		var pipeline db.Pipeline
		var config atc.Config

		BeforeEach(func() {
			var created bool
			var err error
			config = atc.Config{
				ResourceTypes: atc.ResourceTypes{
					{
						Name:   "some-resourceType",
						Type:   "base",
						Source: atc.Source{"some": "repository"},
					},
				},
				Resources: atc.ResourceConfigs{
					{
						Name:   "some-resource",
						Type:   "some-type",
						Source: atc.Source{"some": "repository"},
					},
					{
						Name:   "some-other-resource",
						Type:   "some-type",
						Source: atc.Source{"some": "repository"},
					},
					{
						Name:   "pipeline-resource",
						Type:   "some-resourceType",
						Source: atc.Source{"some": "repository"},
					},
					{
						Name:   "other-pipeline-resource",
						Type:   "some-resourceType",
						Source: atc.Source{"some": "repository"},
					},
				},
				Jobs: atc.JobConfigs{
					{
						Name: "job-using-resource",
						PlanSequence: []atc.Step{
							{
								Config: &atc.GetStep{
									Name: "some-resource",
								},
							},
						},
					},
					{
						Name: "not-using-resource",
					},
				},
			}

			pipeline, created, err = defaultTeam.SavePipeline(
				atc.PipelineRef{Name: "pipeline-with-same-resources"},
				config,
				0,
				false,
			)
			Expect(err).ToNot(HaveOccurred())
			Expect(created).To(BeTrue())
		})

		Context("when the enable global resources flag is set to true", func() {
			BeforeEach(func() {
				atc.EnableGlobalResources = true
			})

			Context("when the resource uses a base resource type with shared version history", func() {
				var (
					resourceScope1 db.ResourceConfigScope
					resourceScope2 db.ResourceConfigScope
					resource1      db.Resource
					resource2      db.Resource
				)

				BeforeEach(func() {
					var found bool
					var err error
					resource1, found, err = pipeline.Resource("some-resource")
					Expect(err).ToNot(HaveOccurred())
					Expect(found).To(BeTrue())

					setupTx, err := dbConn.Begin()
					Expect(err).ToNot(HaveOccurred())

					brt := db.BaseResourceType{
						Name: "some-type",
					}

					_, err = brt.FindOrCreate(setupTx, false)
					Expect(err).NotTo(HaveOccurred())
					Expect(setupTx.Commit()).To(Succeed())

					resourceScope1, err = resource1.SetResourceConfig(atc.Source{"some": "repository"}, atc.VersionedResourceTypes{})
					Expect(err).NotTo(HaveOccurred())

					found, err = resource1.Reload()
					Expect(err).NotTo(HaveOccurred())
					Expect(found).To(BeTrue())
				})

				It("has the resource config id and resource config scope id set on the resource", func() {
					Expect(resourceScope1.Resource()).To(BeNil())
					Expect(resource1.ResourceConfigID()).To(Equal(resourceScope1.ResourceConfig().ID()))
					Expect(resource1.ResourceConfigScopeID()).To(Equal(resourceScope1.ID()))
				})

				Context("when another resource uses the same resource config", func() {
					BeforeEach(func() {
						var found bool
						var err error
						resource2, found, err = pipeline.Resource("some-other-resource")
						Expect(err).ToNot(HaveOccurred())
						Expect(found).To(BeTrue())

						resourceScope2, err = resource2.SetResourceConfig(atc.Source{"some": "repository"}, atc.VersionedResourceTypes{})
						Expect(err).NotTo(HaveOccurred())

						found, err = resource2.Reload()
						Expect(err).NotTo(HaveOccurred())
						Expect(found).To(BeTrue())
					})

					It("has the same resource config id and resource config scope id as the first resource", func() {
						Expect(resource2.ResourceConfigID()).To(Equal(resourceScope2.ResourceConfig().ID()))
						Expect(resource2.ResourceConfigScopeID()).To(Equal(resourceScope2.ID()))
						Expect(resource1.ResourceConfigID()).To(Equal(resource2.ResourceConfigID()))
						Expect(resource1.ResourceConfigScopeID()).To(Equal(resource2.ResourceConfigScopeID()))
					})
				})
			})

			Context("when the resource uses a base resource type that has unique version history", func() {
				var (
					resourceScope1 db.ResourceConfigScope
					resourceScope2 db.ResourceConfigScope
					resource1      db.Resource
					resource2      db.Resource
				)

				BeforeEach(func() {
					var found bool
					var err error
					resource1, found, err = pipeline.Resource("some-resource")
					Expect(err).ToNot(HaveOccurred())
					Expect(found).To(BeTrue())

					setupTx, err := dbConn.Begin()
					Expect(err).ToNot(HaveOccurred())

					brt := db.BaseResourceType{
						Name: "some-type",
					}

					_, err = brt.FindOrCreate(setupTx, true)
					Expect(err).NotTo(HaveOccurred())
					Expect(setupTx.Commit()).To(Succeed())

					resourceScope1, err = resource1.SetResourceConfig(atc.Source{"some": "repository"}, atc.VersionedResourceTypes{})
					Expect(err).NotTo(HaveOccurred())

					found, err = resource1.Reload()
					Expect(err).NotTo(HaveOccurred())
					Expect(found).To(BeTrue())
				})

				It("has the resource config id and resource config scope id set on the resource", func() {
					Expect(resource1.ResourceConfigID()).To(Equal(resourceScope1.ResourceConfig().ID()))
					Expect(resource1.ResourceConfigScopeID()).To(Equal(resourceScope1.ID()))
				})

				Context("when another resource uses the same resource config", func() {
					BeforeEach(func() {
						var found bool
						var err error
						resource2, found, err = pipeline.Resource("some-other-resource")
						Expect(err).ToNot(HaveOccurred())
						Expect(found).To(BeTrue())

						resourceScope2, err = resource2.SetResourceConfig(atc.Source{"some": "repository"}, atc.VersionedResourceTypes{})
						Expect(err).NotTo(HaveOccurred())

						found, err = resource2.Reload()
						Expect(err).NotTo(HaveOccurred())
						Expect(found).To(BeTrue())
					})

					It("has a different resource config scope id than the first resource", func() {
						Expect(resource2.ResourceConfigID()).To(Equal(resourceScope2.ResourceConfig().ID()))
						Expect(resource2.ResourceConfigScopeID()).To(Equal(resourceScope2.ID()))
						Expect(resource1.ResourceConfigID()).To(Equal(resource2.ResourceConfigID()))
						Expect(resource1.ResourceConfigScopeID()).ToNot(Equal(resource2.ResourceConfigScopeID()))
					})
				})
			})
		})

		Context("when the enable global resources flag is set to false, all resources will have a unique history", func() {
			BeforeEach(func() {
				atc.EnableGlobalResources = false
			})

			Context("when the resource uses a base resource type with shared version history", func() {
				var (
					resource1 db.Resource
					resource2 db.Resource
				)

				BeforeEach(func() {
					var found bool
					var err error
					resource1, found, err = pipeline.Resource("some-resource")
					Expect(err).ToNot(HaveOccurred())
					Expect(found).To(BeTrue())

					resource2, found, err = pipeline.Resource("some-other-resource")
					Expect(err).ToNot(HaveOccurred())
					Expect(found).To(BeTrue())

					setupTx, err := dbConn.Begin()
					Expect(err).ToNot(HaveOccurred())

					brt := db.BaseResourceType{
						Name: "some-type",
					}

					_, err = brt.FindOrCreate(setupTx, false)
					Expect(err).NotTo(HaveOccurred())
					Expect(setupTx.Commit()).To(Succeed())

					_, err = resource1.SetResourceConfig(atc.Source{"some": "repository"}, atc.VersionedResourceTypes{})
					Expect(err).NotTo(HaveOccurred())

					_, err = resource2.SetResourceConfig(atc.Source{"some": "repository"}, atc.VersionedResourceTypes{})
					Expect(err).NotTo(HaveOccurred())

					found, err = resource1.Reload()
					Expect(err).NotTo(HaveOccurred())
					Expect(found).To(BeTrue())

					found, err = resource2.Reload()
					Expect(err).NotTo(HaveOccurred())
					Expect(found).To(BeTrue())
				})

				It("has unique version histories", func() {
					Expect(resource1.ResourceConfigScopeID()).ToNot(Equal(resource2.ResourceConfigScopeID()))
				})
			})
		})

		Context("when requesting schedule for setting resource config", func() {
			var resource1 db.Resource

			BeforeEach(func() {
				var err error
				var found bool
				resource1, found, err = pipeline.Resource("some-resource")
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeTrue())

				setupTx, err := dbConn.Begin()
				Expect(err).ToNot(HaveOccurred())

				brt := db.BaseResourceType{
					Name: "some-type",
				}

				_, err = brt.FindOrCreate(setupTx, false)
				Expect(err).NotTo(HaveOccurred())
				Expect(setupTx.Commit()).To(Succeed())
			})

			It("requests schedule on the jobs that use the resource", func() {
				job, found, err := pipeline.Job("job-using-resource")
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeTrue())

				requestedSchedule := job.ScheduleRequestedTime()

				_, err = resource1.SetResourceConfig(atc.Source{"some": "repository"}, atc.VersionedResourceTypes{})
				Expect(err).NotTo(HaveOccurred())

				found, err = job.Reload()
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeTrue())

				Expect(job.ScheduleRequestedTime()).Should(BeTemporally(">", requestedSchedule))
			})

			It("does not request schedule on the jobs that do not use the resource", func() {
				job, found, err := pipeline.Job("not-using-resource")
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeTrue())

				requestedSchedule := job.ScheduleRequestedTime()

				_, err = resource1.SetResourceConfig(atc.Source{"some": "repository"}, atc.VersionedResourceTypes{})
				Expect(err).NotTo(HaveOccurred())

				found, err = job.Reload()
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeTrue())

				Expect(job.ScheduleRequestedTime()).Should(BeTemporally("==", requestedSchedule))
			})
		})
	})

	Describe("ResourceConfigVersion", func() {
		var (
			resource                   db.Resource
			version                    atc.Version
			rcvID                      int
			resourceConfigVersionFound bool
			foundErr                   error
		)

		BeforeEach(func() {
			var err error
			var found bool
			version = atc.Version{"version": "12345"}
			resource, found, err = pipeline.Resource("some-resource")
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())
		})

		JustBeforeEach(func() {
			rcvID, resourceConfigVersionFound, foundErr = resource.ResourceConfigVersionID(version)
		})

		Context("when the version exists", func() {
			var (
				resourceConfigVersion db.ResourceConfigVersion
				resourceScope         db.ResourceConfigScope
			)

			saveVersion := func(version atc.Version) {
				setupTx, err := dbConn.Begin()
				Expect(err).ToNot(HaveOccurred())

				brt := db.BaseResourceType{
					Name: "registry-image",
				}

				_, err = brt.FindOrCreate(setupTx, false)
				Expect(err).ToNot(HaveOccurred())
				Expect(setupTx.Commit()).To(Succeed())

				resourceScope, err = resource.SetResourceConfig(atc.Source{"some": "repository"}, atc.VersionedResourceTypes{})
				Expect(err).ToNot(HaveOccurred())

				err = resourceScope.SaveVersions(nil, []atc.Version{version})
				Expect(err).ToNot(HaveOccurred())

				var found bool
				resourceConfigVersion, found, err = resourceScope.FindVersion(version)
				Expect(found).To(BeTrue())
				Expect(err).ToNot(HaveOccurred())
			}

			Context("when the version is exact matched", func() {
				BeforeEach(func() {
					saveVersion(atc.Version{"version": "12345"})
				})

				It("returns resource config version and true", func() {
					Expect(resourceConfigVersionFound).To(BeTrue())
					Expect(rcvID).To(Equal(resourceConfigVersion.ID()))
					Expect(foundErr).ToNot(HaveOccurred())
				})
			})

			Context("when the version is partially matched", func() {
				BeforeEach(func() {
					saveVersion(atc.Version{"version": "12345", "tag": "1.0.0"})
				})

				It("returns resource config version and true", func() {
					Expect(resourceConfigVersionFound).To(BeTrue())
					Expect(rcvID).To(Equal(resourceConfigVersion.ID()))
					Expect(foundErr).ToNot(HaveOccurred())
				})
			})

			Context("when there are multiple versions partially matched", func() {
				BeforeEach(func() {
					version1 := atc.Version{"version": "12345", "tag": "1.0.0"}
					saveVersion(version1)

					version2 := atc.Version{"version": "12345", "tag": "2.0.0"}
					saveVersion(version2)
				})

				It("returns resource config version with highest order and true", func() {
					Expect(resourceConfigVersionFound).To(BeTrue())
					Expect(rcvID).To(Equal(resourceConfigVersion.ID()))
					Expect(foundErr).ToNot(HaveOccurred())
				})
			})

			Context("when the check order is 0", func() {
				BeforeEach(func() {
					saveVersion(atc.Version{"version": "12345"})

					version = atc.Version{"version": "2"}
					created, err := resource.SaveUncheckedVersion(version, nil, resourceScope.ResourceConfig())
					Expect(err).ToNot(HaveOccurred())
					Expect(created).To(BeTrue())
				})

				It("does not find the resource config version", func() {
					Expect(rcvID).To(Equal(0))
					Expect(resourceConfigVersionFound).To(BeFalse())
					Expect(foundErr).ToNot(HaveOccurred())
				})
			})
		})

		Context("when the version is not found", func() {
			BeforeEach(func() {
				setupTx, err := dbConn.Begin()
				Expect(err).ToNot(HaveOccurred())

				brt := db.BaseResourceType{
					Name: "registry-image",
				}

				_, err = brt.FindOrCreate(setupTx, false)
				Expect(err).NotTo(HaveOccurred())
				Expect(setupTx.Commit()).To(Succeed())
				_, err = resourceConfigFactory.FindOrCreateResourceConfig("registry-image", atc.Source{"some": "repository"}, atc.VersionedResourceTypes{})
				Expect(err).NotTo(HaveOccurred())
			})

			It("returns false when resourceConfig is not found", func() {
				Expect(foundErr).ToNot(HaveOccurred())
				Expect(resourceConfigVersionFound).To(BeFalse())
			})
		})
	})

	Context("Versions", func() {
		var (
			originalVersionSlice []atc.Version
			resource             db.Resource
			resourceScope        db.ResourceConfigScope
		)

		Context("with version filters", func() {
			var filter atc.Version
			var resourceVersions []atc.ResourceVersion

			BeforeEach(func() {
				setupTx, err := dbConn.Begin()
				Expect(err).ToNot(HaveOccurred())

				brt := db.BaseResourceType{
					Name: "git",
				}

				_, err = brt.FindOrCreate(setupTx, false)
				Expect(err).NotTo(HaveOccurred())
				Expect(setupTx.Commit()).To(Succeed())

				var found bool
				resource, found, err = pipeline.Resource("some-other-resource")
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeTrue())

				resourceScope, err = resource.SetResourceConfig(atc.Source{"some": "other-repository"}, atc.VersionedResourceTypes{})
				Expect(err).ToNot(HaveOccurred())

				originalVersionSlice = []atc.Version{
					{"ref": "v0", "commit": "v0"},
					{"ref": "v1", "commit": "v1"},
					{"ref": "v2", "commit": "v2"},
				}

				err = resourceScope.SaveVersions(nil, originalVersionSlice)
				Expect(err).ToNot(HaveOccurred())

				resourceVersions = make([]atc.ResourceVersion, 0)

				for i := 0; i < 3; i++ {
					rcv, found, err := resourceScope.FindVersion(atc.Version{
						"ref":    "v" + strconv.Itoa(i),
						"commit": "v" + strconv.Itoa(i),
					})
					Expect(err).ToNot(HaveOccurred())
					Expect(found).To(BeTrue())

					var metadata []atc.MetadataField

					for _, v := range rcv.Metadata() {
						metadata = append(metadata, atc.MetadataField(v))
					}

					resourceVersion := atc.ResourceVersion{
						ID:       rcv.ID(),
						Version:  atc.Version(rcv.Version()),
						Metadata: metadata,
						Enabled:  true,
					}

					resourceVersions = append(resourceVersions, resourceVersion)
				}
			})

			Context("when filter include one field that matches", func() {
				BeforeEach(func() {
					filter = atc.Version{"ref": "v2"}
				})

				It("return version that matches field filter", func() {
					result, _, found, err := resource.Versions(db.Page{Limit: 10}, filter)
					Expect(err).ToNot(HaveOccurred())
					Expect(found).To(BeTrue())
					Expect(len(result)).To(Equal(1))
					Expect(result[0].Version).To(Equal(resourceVersions[2].Version))
				})
			})

			Context("when filter include one field that doesn't match", func() {
				BeforeEach(func() {
					filter = atc.Version{"ref": "v20"}
				})

				It("return no version", func() {
					result, _, found, err := resource.Versions(db.Page{Limit: 10}, filter)
					Expect(err).ToNot(HaveOccurred())
					Expect(found).To(BeTrue())
					Expect(len(result)).To(Equal(0))
				})
			})

			Context("when filter include two fields that match", func() {
				BeforeEach(func() {
					filter = atc.Version{"ref": "v1", "commit": "v1"}
				})

				It("return version", func() {
					result, _, found, err := resource.Versions(db.Page{Limit: 10}, filter)
					Expect(err).ToNot(HaveOccurred())
					Expect(found).To(BeTrue())
					Expect(len(result)).To(Equal(1))
					Expect(result[0].Version).To(Equal(resourceVersions[1].Version))
				})
			})

			Context("when filter include two fields and one of them does not match", func() {
				BeforeEach(func() {
					filter = atc.Version{"ref": "v1", "commit": "v2"}
				})

				It("return no version", func() {
					result, _, found, err := resource.Versions(db.Page{Limit: 10}, filter)
					Expect(err).ToNot(HaveOccurred())
					Expect(found).To(BeTrue())
					Expect(len(result)).To(Equal(0))
				})
			})
		})

		Context("when resource has versions created in order of check order", func() {
			var resourceVersions []atc.ResourceVersion

			BeforeEach(func() {
				setupTx, err := dbConn.Begin()
				Expect(err).ToNot(HaveOccurred())

				brt := db.BaseResourceType{
					Name: "git",
				}

				_, err = brt.FindOrCreate(setupTx, false)
				Expect(err).NotTo(HaveOccurred())
				Expect(setupTx.Commit()).To(Succeed())

				var found bool
				resource, found, err = pipeline.Resource("some-other-resource")
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeTrue())

				resourceScope, err = resource.SetResourceConfig(atc.Source{"some": "other-repository"}, atc.VersionedResourceTypes{})
				Expect(err).ToNot(HaveOccurred())

				originalVersionSlice = []atc.Version{
					{"ref": "v0"},
					{"ref": "v1"},
					{"ref": "v2"},
					{"ref": "v3"},
					{"ref": "v4"},
					{"ref": "v5"},
					{"ref": "v6"},
					{"ref": "v7"},
					{"ref": "v8"},
					{"ref": "v9"},
				}

				err = resourceScope.SaveVersions(nil, originalVersionSlice)
				Expect(err).ToNot(HaveOccurred())

				resourceVersions = make([]atc.ResourceVersion, 0)

				for i := 0; i < 10; i++ {
					rcv, found, err := resourceScope.FindVersion(atc.Version{"ref": "v" + strconv.Itoa(i)})
					Expect(err).ToNot(HaveOccurred())
					Expect(found).To(BeTrue())

					var metadata []atc.MetadataField

					for _, v := range rcv.Metadata() {
						metadata = append(metadata, atc.MetadataField(v))
					}

					resourceVersion := atc.ResourceVersion{
						ID:       rcv.ID(),
						Version:  atc.Version(rcv.Version()),
						Metadata: metadata,
						Enabled:  true,
					}

					resourceVersions = append(resourceVersions, resourceVersion)
				}

				reloaded, err := resource.Reload()
				Expect(err).ToNot(HaveOccurred())
				Expect(reloaded).To(BeTrue())
			})

			Context("with no from/to", func() {
				It("returns the first page, with the given limit, and a next page", func() {
					historyPage, pagination, found, err := resource.Versions(db.Page{Limit: 2}, nil)
					Expect(err).ToNot(HaveOccurred())
					Expect(found).To(BeTrue())
					Expect(len(historyPage)).To(Equal(2))
					Expect(historyPage[0].Version).To(Equal(resourceVersions[9].Version))
					Expect(historyPage[1].Version).To(Equal(resourceVersions[8].Version))
					Expect(pagination.Newer).To(BeNil())
					Expect(pagination.Older).To(Equal(&db.Page{To: db.NewIntPtr(resourceVersions[7].ID), Limit: 2}))
				})
			})

			Context("with a to that places it in the middle of the versions", func() {
				It("returns the versions, with previous/next pages", func() {
					historyPage, pagination, found, err := resource.Versions(db.Page{To: db.NewIntPtr(resourceVersions[6].ID), Limit: 2}, nil)
					Expect(err).ToNot(HaveOccurred())
					Expect(found).To(BeTrue())
					Expect(len(historyPage)).To(Equal(2))
					Expect(historyPage[0].Version).To(Equal(resourceVersions[6].Version))
					Expect(historyPage[1].Version).To(Equal(resourceVersions[5].Version))
					Expect(pagination.Newer).To(Equal(&db.Page{From: db.NewIntPtr(resourceVersions[7].ID), Limit: 2}))
					Expect(pagination.Older).To(Equal(&db.Page{To: db.NewIntPtr(resourceVersions[4].ID), Limit: 2}))
				})
			})

			Context("with a to that places it to the oldest version", func() {
				It("returns the versions, with no next page", func() {
					historyPage, pagination, found, err := resource.Versions(db.Page{To: db.NewIntPtr(resourceVersions[1].ID), Limit: 2}, nil)
					Expect(err).ToNot(HaveOccurred())
					Expect(found).To(BeTrue())
					Expect(len(historyPage)).To(Equal(2))
					Expect(historyPage[0].Version).To(Equal(resourceVersions[1].Version))
					Expect(historyPage[1].Version).To(Equal(resourceVersions[0].Version))
					Expect(pagination.Newer).To(Equal(&db.Page{From: db.NewIntPtr(resourceVersions[2].ID), Limit: 2}))
					Expect(pagination.Older).To(BeNil())
				})
			})

			Context("with a from that places it in the middle of the versions", func() {
				It("returns the versions, with previous/next pages", func() {
					historyPage, pagination, found, err := resource.Versions(db.Page{From: db.NewIntPtr(resourceVersions[6].ID), Limit: 2}, nil)
					Expect(err).ToNot(HaveOccurred())
					Expect(found).To(BeTrue())
					Expect(len(historyPage)).To(Equal(2))
					Expect(historyPage[0].Version).To(Equal(resourceVersions[7].Version))
					Expect(historyPage[1].Version).To(Equal(resourceVersions[6].Version))
					Expect(pagination.Newer).To(Equal(&db.Page{From: db.NewIntPtr(resourceVersions[8].ID), Limit: 2}))
					Expect(pagination.Older).To(Equal(&db.Page{To: db.NewIntPtr(resourceVersions[5].ID), Limit: 2}))
				})
			})

			Context("with a from that places it at the beginning of the most recent versions", func() {
				It("returns the versions, with no previous page", func() {
					historyPage, pagination, found, err := resource.Versions(db.Page{From: db.NewIntPtr(resourceVersions[8].ID), Limit: 2}, nil)
					Expect(err).ToNot(HaveOccurred())
					Expect(found).To(BeTrue())
					Expect(len(historyPage)).To(Equal(2))
					Expect(historyPage[0].Version).To(Equal(resourceVersions[9].Version))
					Expect(historyPage[1].Version).To(Equal(resourceVersions[8].Version))
					Expect(pagination.Newer).To(BeNil())
					Expect(pagination.Older).To(Equal(&db.Page{To: db.NewIntPtr(resourceVersions[7].ID), Limit: 2}))
				})
			})

			Context("when the version has metadata", func() {
				BeforeEach(func() {
					metadata := []db.ResourceConfigMetadataField{{Name: "name1", Value: "value1"}}

					// save metadata
					_, err := resource.SaveUncheckedVersion(atc.Version(resourceVersions[9].Version), metadata, resourceScope.ResourceConfig())
					Expect(err).ToNot(HaveOccurred())
				})

				It("returns the metadata in the version history", func() {
					historyPage, _, found, err := resource.Versions(db.Page{Limit: 1}, nil)
					Expect(err).ToNot(HaveOccurred())
					Expect(found).To(BeTrue())
					Expect(len(historyPage)).To(Equal(1))
					Expect(historyPage[0].Version).To(Equal(resourceVersions[9].Version))
					Expect(historyPage[0].Metadata).To(Equal([]atc.MetadataField{{Name: "name1", Value: "value1"}}))
				})

				It("maintains existing metadata after same version is saved with no metadata", func() {
					resourceScope, err := resource.SetResourceConfig(atc.Source{"some": "other-repository"}, atc.VersionedResourceTypes{})
					Expect(err).ToNot(HaveOccurred())

					err = resourceScope.SaveVersions(nil, []atc.Version{resourceVersions[9].Version})
					Expect(err).ToNot(HaveOccurred())

					historyPage, _, found, err := resource.Versions(db.Page{Limit: 1}, atc.Version{})
					Expect(err).ToNot(HaveOccurred())
					Expect(found).To(BeTrue())
					Expect(len(historyPage)).To(Equal(1))
					Expect(historyPage[0].Version).To(Equal(resourceVersions[9].Version))
					Expect(historyPage[0].Metadata).To(Equal([]atc.MetadataField{{Name: "name1", Value: "value1"}}))
				})

				It("updates metadata after same version is saved with different metadata", func() {
					newMetadata := []db.ResourceConfigMetadataField{{Name: "name-new", Value: "value-new"}}
					_, err := resource.SaveUncheckedVersion(atc.Version(resourceVersions[9].Version), newMetadata, resourceScope.ResourceConfig())

					historyPage, _, found, err := resource.Versions(db.Page{Limit: 1}, atc.Version{})
					Expect(err).ToNot(HaveOccurred())
					Expect(found).To(BeTrue())
					Expect(len(historyPage)).To(Equal(1))
					Expect(historyPage[0].Version).To(Equal(resourceVersions[9].Version))
					Expect(historyPage[0].Metadata).To(Equal([]atc.MetadataField{{Name: "name-new", Value: "value-new"}}))
				})
			})

			Context("when a version is disabled", func() {
				BeforeEach(func() {
					err := resource.DisableVersion(resourceVersions[9].ID)
					Expect(err).ToNot(HaveOccurred())

					resourceVersions[9].Enabled = false
				})

				It("returns a disabled version", func() {
					historyPage, _, found, err := resource.Versions(db.Page{Limit: 1}, nil)
					Expect(err).ToNot(HaveOccurred())
					Expect(found).To(BeTrue())
					Expect(historyPage).To(ConsistOf([]atc.ResourceVersion{resourceVersions[9]}))
				})
			})

			Context("when the version metadata is updated", func() {
				var metadata db.ResourceConfigMetadataFields

				BeforeEach(func() {
					metadata = []db.ResourceConfigMetadataField{{Name: "name1", Value: "value1"}}

					updated, err := resource.UpdateMetadata(resourceVersions[9].Version, metadata)
					Expect(err).ToNot(HaveOccurred())
					Expect(updated).To(BeTrue())
				})

				It("returns a version with metadata updated", func() {
					historyPage, _, found, err := resource.Versions(db.Page{Limit: 1}, nil)
					Expect(err).ToNot(HaveOccurred())
					Expect(found).To(BeTrue())
					Expect(len(historyPage)).To(Equal(1))
					Expect(historyPage[0].Version).To(Equal(resourceVersions[9].Version))
					Expect(historyPage[0].Metadata).To(Equal([]atc.MetadataField{{Name: "name1", Value: "value1"}}))
				})
			})
		})

		Context("when check orders are different than versions ids", func() {
			var resourceVersions []atc.ResourceVersion

			BeforeEach(func() {
				setupTx, err := dbConn.Begin()
				Expect(err).ToNot(HaveOccurred())

				brt := db.BaseResourceType{
					Name: "git",
				}

				_, err = brt.FindOrCreate(setupTx, false)
				Expect(err).NotTo(HaveOccurred())
				Expect(setupTx.Commit()).To(Succeed())

				var found bool
				resource, found, err = pipeline.Resource("some-other-resource")
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeTrue())

				resourceScope, err = resource.SetResourceConfig(atc.Source{"some": "other-repository"}, atc.VersionedResourceTypes{})
				Expect(err).ToNot(HaveOccurred())

				originalVersionSlice := []atc.Version{
					{"ref": "v1"}, // id: 1, check_order: 1
					{"ref": "v3"}, // id: 2, check_order: 2
					{"ref": "v4"}, // id: 3, check_order: 3
				}

				err = resourceScope.SaveVersions(nil, originalVersionSlice)
				Expect(err).ToNot(HaveOccurred())

				secondVersionSlice := []atc.Version{
					{"ref": "v2"}, // id: 4, check_order: 4
					{"ref": "v3"}, // id: 2, check_order: 5
					{"ref": "v4"}, // id: 3, check_order: 6
				}

				err = resourceScope.SaveVersions(nil, secondVersionSlice)
				Expect(err).ToNot(HaveOccurred())

				for i := 1; i < 5; i++ {
					rcv, found, err := resourceScope.FindVersion(atc.Version{"ref": "v" + strconv.Itoa(i)})
					Expect(err).ToNot(HaveOccurred())
					Expect(found).To(BeTrue())

					var metadata []atc.MetadataField

					for _, v := range rcv.Metadata() {
						metadata = append(metadata, atc.MetadataField(v))
					}

					resourceVersion := atc.ResourceVersion{
						ID:       rcv.ID(),
						Version:  atc.Version(rcv.Version()),
						Metadata: metadata,
						Enabled:  true,
					}

					resourceVersions = append(resourceVersions, resourceVersion)
				}

				// ids ordered by check order now: [3, 2, 4, 1]
			})

			Context("with no from/to", func() {
				It("returns versions ordered by check order", func() {
					historyPage, pagination, found, err := resource.Versions(db.Page{Limit: 4}, nil)
					Expect(err).ToNot(HaveOccurred())
					Expect(found).To(BeTrue())
					Expect(historyPage).To(HaveLen(4))
					Expect(historyPage[0].Version).To(Equal(resourceVersions[3].Version))
					Expect(historyPage[1].Version).To(Equal(resourceVersions[2].Version))
					Expect(historyPage[2].Version).To(Equal(resourceVersions[1].Version))
					Expect(historyPage[3].Version).To(Equal(resourceVersions[0].Version))
					Expect(pagination.Newer).To(BeNil())
					Expect(pagination.Older).To(BeNil())
				})
			})

			Context("with from", func() {
				It("returns the versions, with previous/next pages including from", func() {
					historyPage, pagination, found, err := resource.Versions(db.Page{From: db.NewIntPtr(resourceVersions[1].ID), Limit: 2}, nil)
					Expect(err).ToNot(HaveOccurred())
					Expect(found).To(BeTrue())
					Expect(historyPage).To(HaveLen(2))
					Expect(historyPage[0].Version).To(Equal(resourceVersions[2].Version))
					Expect(historyPage[1].Version).To(Equal(resourceVersions[1].Version))
					Expect(pagination.Newer).To(Equal(&db.Page{From: db.NewIntPtr(resourceVersions[3].ID), Limit: 2}))
					Expect(pagination.Older).To(Equal(&db.Page{To: db.NewIntPtr(resourceVersions[0].ID), Limit: 2}))
				})
			})

			Context("with to", func() {
				It("returns the builds, with previous/next pages including to", func() {
					historyPage, pagination, found, err := resource.Versions(db.Page{To: db.NewIntPtr(resourceVersions[2].ID), Limit: 2}, nil)
					Expect(err).ToNot(HaveOccurred())
					Expect(found).To(BeTrue())
					Expect(historyPage).To(HaveLen(2))
					Expect(historyPage[0].Version).To(Equal(resourceVersions[2].Version))
					Expect(historyPage[1].Version).To(Equal(resourceVersions[1].Version))
					Expect(pagination.Newer).To(Equal(&db.Page{From: db.NewIntPtr(resourceVersions[3].ID), Limit: 2}))
					Expect(pagination.Older).To(Equal(&db.Page{To: db.NewIntPtr(resourceVersions[0].ID), Limit: 2}))
				})
			})
		})

		Context("when resource has a version with check order of 0", func() {
			var resource db.Resource

			BeforeEach(func() {
				setupTx, err := dbConn.Begin()
				Expect(err).ToNot(HaveOccurred())

				brt := db.BaseResourceType{
					Name: "git",
				}

				_, err = brt.FindOrCreate(setupTx, false)
				Expect(err).NotTo(HaveOccurred())
				Expect(setupTx.Commit()).To(Succeed())

				var found bool
				resource, found, err = pipeline.Resource("some-other-resource")
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeTrue())

				resourceScope, err := resource.SetResourceConfig(atc.Source{"some": "other-repository"}, atc.VersionedResourceTypes{})
				Expect(err).ToNot(HaveOccurred())

				created, err := resource.SaveUncheckedVersion(atc.Version{"version": "not-returned"}, nil, resourceScope.ResourceConfig())
				Expect(err).ToNot(HaveOccurred())
				Expect(created).To(BeTrue())
			})

			It("does not return the version", func() {
				historyPage, pagination, found, err := resource.Versions(db.Page{Limit: 2}, nil)
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeTrue())
				Expect(historyPage).To(BeNil())
				Expect(pagination).To(Equal(db.Pagination{Newer: nil, Older: nil}))
			})
		})
	})

	Describe("PinVersion/UnpinVersion", func() {
		var (
			resource      db.Resource
			resourceScope db.ResourceConfigScope
			resID         int
		)

		BeforeEach(func() {
			var found bool
			var err error
			resource, found, err = pipeline.Resource("some-other-resource")
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())

			setupTx, err := dbConn.Begin()
			Expect(err).ToNot(HaveOccurred())

			brt := db.BaseResourceType{
				Name: "git",
			}

			_, err = brt.FindOrCreate(setupTx, false)
			Expect(err).NotTo(HaveOccurred())
			Expect(setupTx.Commit()).To(Succeed())

			resourceScope, err = resource.SetResourceConfig(atc.Source{"some": "other-repository"}, atc.VersionedResourceTypes{})
			Expect(err).ToNot(HaveOccurred())

			err = resourceScope.SaveVersions(nil, []atc.Version{
				{"version": "v1"},
				{"version": "v2"},
				{"version": "v3"},
			})
			Expect(err).ToNot(HaveOccurred())

			resConf, found, err := resourceScope.FindVersion(atc.Version{"version": "v1"})
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())
			resID = resConf.ID()
		})

		Context("when we use an invalid version id (does not exist)", func() {
			var (
				pinnedVersion atc.Version
			)
			BeforeEach(func() {
				found, err := resource.PinVersion(resID)
				Expect(found).To(BeTrue())
				Expect(err).ToNot(HaveOccurred())

				found, err = resource.Reload()
				Expect(found).To(BeTrue())
				Expect(err).ToNot(HaveOccurred())

				Expect(resource.CurrentPinnedVersion()).To(Equal(resource.APIPinnedVersion()))
				pinnedVersion = resource.APIPinnedVersion()
			})

			It("returns not found and does not update anything", func() {
				found, err := resource.PinVersion(-1)
				Expect(found).To(BeFalse())
				Expect(err).To(HaveOccurred())

				Expect(resource.APIPinnedVersion()).To(Equal(pinnedVersion))
			})
		})

		Context("when requesting schedule for version pinning", func() {
			It("requests schedule on all jobs using the resource", func() {
				job, found, err := pipeline.Job("job-using-resource")
				Expect(found).To(BeTrue())
				Expect(err).ToNot(HaveOccurred())

				requestedSchedule := job.ScheduleRequestedTime()

				found, err = resource.PinVersion(resID)
				Expect(found).To(BeTrue())
				Expect(err).ToNot(HaveOccurred())

				found, err = job.Reload()
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeTrue())

				Expect(job.ScheduleRequestedTime()).Should(BeTemporally(">", requestedSchedule))
			})

			It("does not request schedule on jobs that do not use the resource", func() {
				job, found, err := pipeline.Job("not-using-resource")
				Expect(found).To(BeTrue())
				Expect(err).ToNot(HaveOccurred())

				requestedSchedule := job.ScheduleRequestedTime()

				found, err = resource.PinVersion(resID)
				Expect(found).To(BeTrue())
				Expect(err).ToNot(HaveOccurred())

				found, err = job.Reload()
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeTrue())

				Expect(job.ScheduleRequestedTime()).Should(BeTemporally("==", requestedSchedule))
			})
		})

		Context("when we pin a resource to a version", func() {
			BeforeEach(func() {
				found, err := resource.PinVersion(resID)
				Expect(found).To(BeTrue())
				Expect(err).ToNot(HaveOccurred())

				found, err = resource.Reload()
				Expect(found).To(BeTrue())
				Expect(err).ToNot(HaveOccurred())
			})

			Context("when the resource is not pinned", func() {
				It("sets the api pinned version", func() {
					Expect(resource.APIPinnedVersion()).To(Equal(atc.Version{"version": "v1"}))
					Expect(resource.CurrentPinnedVersion()).To(Equal(resource.APIPinnedVersion()))
				})
			})

			Context("when the resource is pinned by another version already", func() {
				BeforeEach(func() {
					resConf, found, err := resourceScope.FindVersion(atc.Version{"version": "v3"})
					Expect(err).ToNot(HaveOccurred())
					Expect(found).To(BeTrue())
					resID = resConf.ID()

					found, err = resource.PinVersion(resID)
					Expect(found).To(BeTrue())
					Expect(err).ToNot(HaveOccurred())

					found, err = resource.Reload()
					Expect(found).To(BeTrue())
					Expect(err).ToNot(HaveOccurred())
				})

				It("switch the pin to given version", func() {
					Expect(resource.APIPinnedVersion()).To(Equal(atc.Version{"version": "v3"}))
					Expect(resource.CurrentPinnedVersion()).To(Equal(resource.APIPinnedVersion()))
				})
			})

			Context("when we set the pin comment on a resource", func() {
				BeforeEach(func() {
					err := resource.SetPinComment("foo")
					Expect(err).ToNot(HaveOccurred())
					reload, err := resource.Reload()
					Expect(reload).To(BeTrue())
					Expect(err).ToNot(HaveOccurred())
				})

				It("should set the pin comment", func() {
					Expect(resource.PinComment()).To(Equal("foo"))
				})
			})

			Context("when requesting schedule for version unpinning", func() {
				It("requests schedule on all jobs using the resource", func() {
					job, found, err := pipeline.Job("job-using-resource")
					Expect(found).To(BeTrue())
					Expect(err).ToNot(HaveOccurred())

					requestedSchedule := job.ScheduleRequestedTime()

					err = resource.UnpinVersion()
					Expect(err).ToNot(HaveOccurred())

					found, err = job.Reload()
					Expect(found).To(BeTrue())
					Expect(err).ToNot(HaveOccurred())

					Expect(job.ScheduleRequestedTime()).Should(BeTemporally(">", requestedSchedule))
				})

				It("does not request schedule on jobs that do not use the resource", func() {
					job, found, err := pipeline.Job("not-using-resource")
					Expect(found).To(BeTrue())
					Expect(err).ToNot(HaveOccurred())

					requestedSchedule := job.ScheduleRequestedTime()

					err = resource.UnpinVersion()
					Expect(err).ToNot(HaveOccurred())

					Expect(job.ScheduleRequestedTime()).Should(BeTemporally("==", requestedSchedule))
				})
			})

			Context("when we unpin a resource to a version", func() {
				BeforeEach(func() {
					err := resource.UnpinVersion()
					Expect(err).ToNot(HaveOccurred())

					found, err := resource.Reload()
					Expect(found).To(BeTrue())
					Expect(err).ToNot(HaveOccurred())
				})

				It("sets the api pinned version to nil", func() {
					Expect(resource.APIPinnedVersion()).To(BeNil())
					Expect(resource.CurrentPinnedVersion()).To(BeNil())
				})

				It("unsets the pin comment", func() {
					Expect(resource.PinComment()).To(BeEmpty())
				})
			})
		})

		Context("when we pin a resource that is already pinned to a version (through the config)", func() {
			var resConf db.ResourceConfigVersion

			BeforeEach(func() {
				var found bool
				var err error
				resource, found, err = pipeline.Resource("some-resource")
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeTrue())

				setupTx, err := dbConn.Begin()
				Expect(err).ToNot(HaveOccurred())

				brt := db.BaseResourceType{
					Name: "registry-image",
				}

				_, err = brt.FindOrCreate(setupTx, false)
				Expect(err).NotTo(HaveOccurred())
				Expect(setupTx.Commit()).To(Succeed())

				resourceScope, err := resource.SetResourceConfig(atc.Source{"some": "repository"}, atc.VersionedResourceTypes{})
				Expect(err).ToNot(HaveOccurred())

				err = resourceScope.SaveVersions(nil, []atc.Version{
					{"version": "v1"},
					{"version": "v2"},
					{"version": "v3"},
				})
				Expect(err).ToNot(HaveOccurred())

				resConf, found, err = resourceScope.FindVersion(atc.Version{"version": "v1"})
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeTrue())
			})

			It("should fail to update the pinned version", func() {
				found, err := resource.PinVersion(resConf.ID())
				Expect(found).To(BeFalse())
				Expect(err).To(Equal(db.ErrPinnedThroughConfig))
			})
		})
	})

	Describe("Public", func() {
		var (
			resource db.Resource
			found    bool
			err      error
		)

		Context("when public is not set in the config", func() {
			BeforeEach(func() {
				resource, found, err = pipeline.Resource("some-resource")
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeTrue())
			})

			It("returns false", func() {
				Expect(resource.Public()).To(BeFalse())
			})
		})

		Context("when public is set to true in the config", func() {
			BeforeEach(func() {
				resource, found, err = pipeline.Resource("some-other-resource")
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeTrue())
			})

			It("returns true", func() {
				Expect(resource.Public()).To(BeTrue())
			})
		})

		Context("when public is set to false in the config", func() {
			BeforeEach(func() {
				resource, found, err = pipeline.Resource("some-secret-resource")
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeTrue())
			})

			It("returns false", func() {
				Expect(resource.Public()).To(BeFalse())
			})
		})
	})
})
