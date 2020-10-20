package db_test

import (
	"context"
	"strconv"
	"time"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/db/dbtest"
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
						DisplayName:  "Some resource",
						Type:         "registry-image",
						WebhookToken: "some-token",
						Source:       atc.Source{"some": "repository"},
						Version:      atc.Version{"ref": "abcdef"},
						CheckTimeout: "999m",
					},
					{
						Name:        "some-other-resource",
						DisplayName: "Some other resource",
						Public:      true,
						Type:        "git",
						Source:      atc.Source{"some": "other-repository"},
					},
					{
						Name:        "some-secret-resource",
						DisplayName: "Some secret resource :o",
						Public:      false,
						Type:        "git",
						Source:      atc.Source{"some": "((secret-repository))"},
					},
					{
						Name:         "some-resource-custom-check",
						DisplayName:  "Some resource with custom check",
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
					Expect(r.DisplayName()).To(Equal("Some resource"))
					Expect(r.Type()).To(Equal("registry-image"))
					Expect(r.Source()).To(Equal(atc.Source{"some": "repository"}))
					Expect(r.ConfigPinnedVersion()).To(Equal(atc.Version{"ref": "abcdef"}))
					Expect(r.CurrentPinnedVersion()).To(Equal(r.ConfigPinnedVersion()))
					Expect(r.HasWebhook()).To(BeTrue())
				case "some-other-resource":
					Expect(r.DisplayName()).To(Equal("Some other resource"))
					Expect(r.Type()).To(Equal("git"))
					Expect(r.Source()).To(Equal(atc.Source{"some": "other-repository"}))
					Expect(r.HasWebhook()).To(BeFalse())
				case "some-secret-resource":
					Expect(r.DisplayName()).To(Equal("Some secret resource :o"))
					Expect(r.Type()).To(Equal("git"))
					Expect(r.Source()).To(Equal(atc.Source{"some": "((secret-repository))"}))
					Expect(r.HasWebhook()).To(BeFalse())
				case "some-resource-custom-check":
					Expect(r.DisplayName()).To(Equal("Some resource with custom check"))
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
			scenario *dbtest.Scenario
		)

		Context("when the resource exists", func() {
			BeforeEach(func() {
				scenario = dbtest.Setup(
					builder.WithPipeline(atc.Config{
						Resources: atc.ResourceConfigs{
							{
								Name:   "some-resource",
								Type:   "some-base-resource-type",
								Source: atc.Source{"some": "repository"},
							},
						},
					}),
				)
			})

			It("returns the resource", func() {
				Expect(scenario.Resource("some-resource").Name()).To(Equal("some-resource"))
				Expect(scenario.Resource("some-resource").Type()).To(Equal("some-base-resource-type"))
				Expect(scenario.Resource("some-resource").Source()).To(Equal(atc.Source{"some": "repository"}))
			})
		})

		Context("when the resource does not exist", func() {
			var resource db.Resource
			var found bool
			var err error

			BeforeEach(func() {
				scenario = dbtest.Setup(
					builder.WithPipeline(atc.Config{
						Resources: atc.ResourceConfigs{
							{
								Name:   "some-resource",
								Type:   "some-base-resource-type",
								Source: atc.Source{"some": "repository"},
							},
						},
					}),
				)

				resource, found, err = scenario.Pipeline.Resource("bonkers")
				Expect(err).ToNot(HaveOccurred())
			})

			It("returns nil", func() {
				Expect(found).To(BeFalse())
				Expect(resource).To(BeNil())
			})
		})
	})

	Describe("Enable/Disable Version", func() {
		var scenario *dbtest.Scenario

		BeforeEach(func() {
			scenario = dbtest.Setup(
				builder.WithPipeline(atc.Config{
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
					Resources: atc.ResourceConfigs{
						{
							Name:   "some-other-resource",
							Type:   "some-base-resource-type",
							Source: atc.Source{"some": "other-repository"},
						},
					},
				}),
				builder.WithResourceVersions("some-other-resource", atc.Version{"disabled": "version"}),
			)
		})

		Context("when disabling a version that exists", func() {
			var disableErr error
			var requestedSchedule1 time.Time
			var requestedSchedule2 time.Time

			BeforeEach(func() {
				requestedSchedule1 = scenario.Job("job-using-resource").ScheduleRequestedTime()
				requestedSchedule2 = scenario.Job("not-using-resource").ScheduleRequestedTime()

				scenario.Run(builder.WithDisabledVersion("some-other-resource", atc.Version{"disabled": "version"}))
			})

			It("successfully disables the version", func() {
				Expect(disableErr).ToNot(HaveOccurred())

				versions, _, found, err := scenario.Resource("some-other-resource").Versions(db.Page{Limit: 3}, nil)
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeTrue())
				Expect(versions).To(HaveLen(1))
				Expect(versions[0].Version).To(Equal(atc.Version{"disabled": "version"}))
				Expect(versions[0].Enabled).To(BeFalse())
			})

			It("requests schedule on the jobs using that resource", func() {
				found, err := scenario.Job("job-using-resource").Reload()
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeTrue())

				Expect(scenario.Job("job-using-resource").ScheduleRequestedTime()).Should(BeTemporally(">", requestedSchedule1))
			})

			It("does not request schedule on jobs that do not use the resource", func() {
				found, err := scenario.Job("not-using-resource").Reload()
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeTrue())

				Expect(scenario.Job("not-using-resource").ScheduleRequestedTime()).Should(BeTemporally("==", requestedSchedule2))
			})

			Context("when enabling that version", func() {
				BeforeEach(func() {
					scenario.Run(builder.WithEnabledVersion("some-other-resource", atc.Version{"disabled": "version"}))
				})

				It("successfully enables the version", func() {
					versions, _, found, err := scenario.Resource("some-other-resource").Versions(db.Page{Limit: 3}, nil)
					Expect(err).ToNot(HaveOccurred())
					Expect(found).To(BeTrue())
					Expect(versions).To(HaveLen(1))
					Expect(versions[0].Version).To(Equal(atc.Version{"disabled": "version"}))
					Expect(versions[0].Enabled).To(BeTrue())
				})

				It("request schedule on the jobs using that resource", func() {
					found, err := scenario.Job("job-using-resource").Reload()
					Expect(err).NotTo(HaveOccurred())
					Expect(found).To(BeTrue())

					Expect(scenario.Job("job-using-resource").ScheduleRequestedTime()).Should(BeTemporally(">", requestedSchedule1))
				})

				It("does not request schedule on jobs that do not use the resource", func() {
					found, err := scenario.Job("not-using-resource").Reload()
					Expect(err).NotTo(HaveOccurred())
					Expect(found).To(BeTrue())

					Expect(scenario.Job("not-using-resource").ScheduleRequestedTime()).Should(BeTemporally("==", requestedSchedule2))
				})
			})
		})

		Context("when disabling version that does not exist", func() {
			var disableErr error
			BeforeEach(func() {
				resource, found, err := scenario.Pipeline.Resource("some-other-resource")
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
				resource, found, err := scenario.Pipeline.Resource("some-other-resource")
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeTrue())

				enableError = resource.EnableVersion(scenario.ResourceVersion("some-other-resource", atc.Version{"disabled": "version"}).ID())
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
			Expect(resource.CheckPlan(atc.Version{"some": "version"}, time.Minute, resourceTypes, defaults)).To(Equal(atc.CheckPlan{
				Name:    resource.Name(),
				Type:    resource.Type(),
				Source:  defaults.Merge(resource.Source()),
				Tags:    resource.Tags(),
				Timeout: "999m",

				FromVersion: atc.Version{"some": "version"},

				Interval: "1m0s",

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

		It("creates a build for a resource", func() {
			Expect(created).To(BeTrue())
			Expect(build).ToNot(BeNil())
			Expect(build.Name()).To(Equal(db.CheckBuildName))
			Expect(build.ResourceID()).To(Equal(defaultResource.ID()))
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
				Name:                 db.CheckBuildName,
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

		Context("when another build already exists", func() {
			var prevBuild db.Build

			BeforeEach(func() {
				var err error
				var prevCreated bool
				prevBuild, prevCreated, err = defaultResource.CreateBuild(ctx, false)
				Expect(err).ToNot(HaveOccurred())
				Expect(prevCreated).To(BeTrue())
			})

			It("does not create the second build", func() {
				Expect(created).To(BeFalse())
			})

			Context("when manually triggered", func() {
				BeforeEach(func() {
					manuallyTriggered = true
				})

				It("creates a manually triggered resource build", func() {
					Expect(created).To(BeTrue())
					Expect(build.IsManuallyTriggered()).To(BeTrue())
					Expect(build.ResourceID()).To(Equal(defaultResource.ID()))
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
						Name:                 db.CheckBuildName,
						Status:               atc.StatusStarted,
						StartTime:            build.StartTime().Unix(),
						TeamName:             defaultTeam.Name(),
						PipelineID:           defaultPipeline.ID(),
						PipelineName:         defaultPipeline.Name(),
						PipelineInstanceVars: defaultPipeline.InstanceVars(),
					}))
				})
			})

			Context("when the previous build is finished", func() {
				BeforeEach(func() {
					Expect(prevBuild.Finish(db.BuildStatusSucceeded)).To(Succeed())
				})

				It("creates the build", func() {
					Expect(created).To(BeTrue())
					Expect(build.ResourceID()).To(Equal(defaultResource.ID()))
				})

				It("deletes the previous build", func() {
					found, err := prevBuild.Reload()
					Expect(err).ToNot(HaveOccurred())
					Expect(found).To(BeFalse())
				})
			})
		})
	})

	Context("Versions", func() {
		var (
			scenario *dbtest.Scenario
		)

		Context("with version filters", func() {
			var filter atc.Version
			var resourceVersions []atc.ResourceVersion

			BeforeEach(func() {
				scenario = dbtest.Setup(
					builder.WithPipeline(atc.Config{
						Resources: atc.ResourceConfigs{
							{
								Name:   "some-resource",
								Type:   "some-base-resource-type",
								Source: atc.Source{"some": "repository"},
							},
						},
					}),
					builder.WithResourceVersions(
						"some-resource",
						atc.Version{"ref": "v0", "commit": "v0"},
						atc.Version{"ref": "v1", "commit": "v1"},
						atc.Version{"ref": "v2", "commit": "v2"},
					),
				)

				resourceVersions = make([]atc.ResourceVersion, 0)

				for i := 0; i < 3; i++ {
					rcv := scenario.ResourceVersion("some-resource", atc.Version{
						"ref":    "v" + strconv.Itoa(i),
						"commit": "v" + strconv.Itoa(i),
					})

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
					result, _, found, err := scenario.Resource("some-resource").Versions(db.Page{Limit: 10}, filter)
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
					result, _, found, err := scenario.Resource("some-resource").Versions(db.Page{Limit: 10}, filter)
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
					result, _, found, err := scenario.Resource("some-resource").Versions(db.Page{Limit: 10}, filter)
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
					result, _, found, err := scenario.Resource("some-resource").Versions(db.Page{Limit: 10}, filter)
					Expect(err).ToNot(HaveOccurred())
					Expect(found).To(BeTrue())
					Expect(len(result)).To(Equal(0))
				})
			})
		})

		Context("when resource has versions created in order of check order", func() {
			var resourceVersions []atc.ResourceVersion

			BeforeEach(func() {
				scenario = dbtest.Setup(
					builder.WithPipeline(atc.Config{
						Resources: atc.ResourceConfigs{
							{
								Name:   "some-resource",
								Type:   "some-base-resource-type",
								Source: atc.Source{"some": "repository"},
							},
						},
					}),
					builder.WithResourceVersions(
						"some-resource",
						atc.Version{"ref": "v0"},
						atc.Version{"ref": "v1"},
						atc.Version{"ref": "v2"},
						atc.Version{"ref": "v3"},
						atc.Version{"ref": "v4"},
						atc.Version{"ref": "v5"},
						atc.Version{"ref": "v6"},
						atc.Version{"ref": "v7"},
						atc.Version{"ref": "v8"},
						atc.Version{"ref": "v9"},
					),
				)

				resourceVersions = make([]atc.ResourceVersion, 0)

				for i := 0; i < 10; i++ {
					rcv := scenario.ResourceVersion("some-resource", atc.Version{"ref": "v" + strconv.Itoa(i)})

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

			Context("with no from/to", func() {
				It("returns the first page, with the given limit, and a next page", func() {
					historyPage, pagination, found, err := scenario.Resource("some-resource").Versions(db.Page{Limit: 2}, nil)
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
					historyPage, pagination, found, err := scenario.Resource("some-resource").Versions(db.Page{To: db.NewIntPtr(resourceVersions[6].ID), Limit: 2}, nil)
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
					historyPage, pagination, found, err := scenario.Resource("some-resource").Versions(db.Page{To: db.NewIntPtr(resourceVersions[1].ID), Limit: 2}, nil)
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
					historyPage, pagination, found, err := scenario.Resource("some-resource").Versions(db.Page{From: db.NewIntPtr(resourceVersions[6].ID), Limit: 2}, nil)
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
					historyPage, pagination, found, err := scenario.Resource("some-resource").Versions(db.Page{From: db.NewIntPtr(resourceVersions[8].ID), Limit: 2}, nil)
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

					scenario.Run(builder.WithVersionMetadata("some-resource", atc.Version(resourceVersions[9].Version), metadata))
				})

				It("returns the metadata in the version history", func() {
					historyPage, _, found, err := scenario.Resource("some-resource").Versions(db.Page{Limit: 1}, nil)
					Expect(err).ToNot(HaveOccurred())
					Expect(found).To(BeTrue())
					Expect(len(historyPage)).To(Equal(1))
					Expect(historyPage[0].Version).To(Equal(resourceVersions[9].Version))
					Expect(historyPage[0].Metadata).To(Equal([]atc.MetadataField{{Name: "name1", Value: "value1"}}))
				})

				It("maintains existing metadata after same version is saved with no metadata", func() {
					scenario.Run(builder.WithResourceVersions("some-resource", atc.Version(resourceVersions[9].Version)))

					historyPage, _, found, err := scenario.Resource("some-resource").Versions(db.Page{Limit: 1}, atc.Version{})
					Expect(err).ToNot(HaveOccurred())
					Expect(found).To(BeTrue())
					Expect(len(historyPage)).To(Equal(1))
					Expect(historyPage[0].Version).To(Equal(resourceVersions[9].Version))
					Expect(historyPage[0].Metadata).To(Equal([]atc.MetadataField{{Name: "name1", Value: "value1"}}))
				})

				It("updates metadata after same version is saved with different metadata", func() {
					newMetadata := []db.ResourceConfigMetadataField{{Name: "name-new", Value: "value-new"}}
					scenario.Run(builder.WithVersionMetadata("some-resource", atc.Version(resourceVersions[9].Version), newMetadata))

					historyPage, _, found, err := scenario.Resource("some-resource").Versions(db.Page{Limit: 1}, atc.Version{})
					Expect(err).ToNot(HaveOccurred())
					Expect(found).To(BeTrue())
					Expect(len(historyPage)).To(Equal(1))
					Expect(historyPage[0].Version).To(Equal(resourceVersions[9].Version))
					Expect(historyPage[0].Metadata).To(Equal([]atc.MetadataField{{Name: "name-new", Value: "value-new"}}))
				})
			})

			Context("when a version is disabled", func() {
				BeforeEach(func() {
					scenario.Run(builder.WithDisabledVersion("some-resource", resourceVersions[9].Version))

					resourceVersions[9].Enabled = false
				})

				It("returns a disabled version", func() {
					historyPage, _, found, err := scenario.Resource("some-resource").Versions(db.Page{Limit: 1}, nil)
					Expect(err).ToNot(HaveOccurred())
					Expect(found).To(BeTrue())
					Expect(historyPage).To(ConsistOf([]atc.ResourceVersion{resourceVersions[9]}))
				})
			})

			Context("when the version metadata is updated", func() {
				var metadata db.ResourceConfigMetadataFields

				BeforeEach(func() {
					metadata = []db.ResourceConfigMetadataField{{Name: "name1", Value: "value1"}}

					scenario.Run(builder.WithVersionMetadata("some-resource", atc.Version(resourceVersions[9].Version), metadata))
				})

				It("returns a version with metadata updated", func() {
					historyPage, _, found, err := scenario.Resource("some-resource").Versions(db.Page{Limit: 1}, nil)
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
				scenario = dbtest.Setup(
					builder.WithPipeline(atc.Config{
						Resources: atc.ResourceConfigs{
							{
								Name:   "some-resource",
								Type:   "some-base-resource-type",
								Source: atc.Source{"some": "repository"},
							},
						},
					}),
					builder.WithResourceVersions(
						"some-resource",
						atc.Version{"ref": "v1"}, // id: 1, check_order: 1
						atc.Version{"ref": "v3"}, // id: 2, check_order: 2
						atc.Version{"ref": "v4"}, // id: 3, check_order: 3
					),
					builder.WithResourceVersions(
						"some-resource",
						atc.Version{"ref": "v2"}, // id: 4, check_order: 4
						atc.Version{"ref": "v3"}, // id: 2, check_order: 5
						atc.Version{"ref": "v4"}, // id: 3, check_order: 6
					),
				)

				for i := 1; i < 5; i++ {
					rcv := scenario.ResourceVersion("some-resource", atc.Version{"ref": "v" + strconv.Itoa(i)})

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
					historyPage, pagination, found, err := scenario.Resource("some-resource").Versions(db.Page{Limit: 4}, nil)
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
					historyPage, pagination, found, err := scenario.Resource("some-resource").Versions(db.Page{From: db.NewIntPtr(resourceVersions[1].ID), Limit: 2}, nil)
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
					historyPage, pagination, found, err := scenario.Resource("some-resource").Versions(db.Page{To: db.NewIntPtr(resourceVersions[2].ID), Limit: 2}, nil)
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
			BeforeEach(func() {
				scenario = dbtest.Setup(
					builder.WithPipeline(atc.Config{
						Resources: atc.ResourceConfigs{
							{
								Name:   "some-resource",
								Type:   "some-base-resource-type",
								Source: atc.Source{"some": "repository"},
							},
						},
					}),
					builder.WithResourceVersions("some-resource"),
					builder.WithVersionMetadata("some-resource", atc.Version{"version": "not-returned"}, nil), // save unchecked version
				)
			})

			It("does not return the version", func() {
				historyPage, pagination, found, err := scenario.Resource("some-resource").Versions(db.Page{Limit: 2}, nil)
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeTrue())
				Expect(historyPage).To(BeNil())
				Expect(pagination).To(Equal(db.Pagination{Newer: nil, Older: nil}))
			})
		})
	})

	Describe("PinVersion/UnpinVersion", func() {
		var (
			scenario *dbtest.Scenario
		)

		BeforeEach(func() {
			scenario = dbtest.Setup(
				builder.WithPipeline(atc.Config{
					Resources: atc.ResourceConfigs{
						{
							Name:   "some-resource",
							Type:   "some-base-resource-type",
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
				}),
				builder.WithResourceVersions(
					"some-resource",
					atc.Version{"version": "v1"},
					atc.Version{"version": "v2"},
					atc.Version{"version": "v3"},
				),
			)
		})

		Context("when we use an invalid version id (does not exist)", func() {
			var (
				pinnedVersion atc.Version
			)

			BeforeEach(func() {
				found, err := scenario.Resource("some-resource").PinVersion(scenario.ResourceVersion("some-resource", atc.Version{"version": "v1"}).ID())
				Expect(found).To(BeTrue())
				Expect(err).ToNot(HaveOccurred())

				Expect(scenario.Resource("some-resource").CurrentPinnedVersion()).To(Equal(scenario.Resource("some-resource").APIPinnedVersion()))
				pinnedVersion = scenario.Resource("some-resource").APIPinnedVersion()
			})

			It("returns not found and does not update anything", func() {
				found, err := scenario.Resource("some-resource").PinVersion(-1)
				Expect(found).To(BeFalse())
				Expect(err).To(HaveOccurred())

				Expect(scenario.Resource("some-resource").APIPinnedVersion()).To(Equal(pinnedVersion))
			})
		})

		Context("when requesting schedule for version pinning", func() {
			It("requests schedule on all jobs using the resource", func() {
				requestedSchedule := scenario.Job("job-using-resource").ScheduleRequestedTime()

				found, err := scenario.Resource("some-resource").PinVersion(scenario.ResourceVersion("some-resource", atc.Version{"version": "v1"}).ID())
				Expect(found).To(BeTrue())
				Expect(err).ToNot(HaveOccurred())

				Expect(scenario.Job("job-using-resource").ScheduleRequestedTime()).Should(BeTemporally(">", requestedSchedule))
			})

			It("does not request schedule on jobs that do not use the resource", func() {
				requestedSchedule := scenario.Job("not-using-resource").ScheduleRequestedTime()

				found, err := scenario.Resource("some-resource").PinVersion(scenario.ResourceVersion("some-resource", atc.Version{"version": "v1"}).ID())
				Expect(found).To(BeTrue())
				Expect(err).ToNot(HaveOccurred())

				Expect(scenario.Job("not-using-resource").ScheduleRequestedTime()).Should(BeTemporally("==", requestedSchedule))
			})
		})

		Context("when we pin a resource to a version", func() {
			BeforeEach(func() {
				found, err := scenario.Resource("some-resource").PinVersion(scenario.ResourceVersion("some-resource", atc.Version{"version": "v1"}).ID())
				Expect(found).To(BeTrue())
				Expect(err).ToNot(HaveOccurred())
			})

			Context("when the resource is not pinned", func() {
				It("sets the api pinned version", func() {
					Expect(scenario.Resource("some-resource").APIPinnedVersion()).To(Equal(atc.Version{"version": "v1"}))
					Expect(scenario.Resource("some-resource").CurrentPinnedVersion()).To(Equal(scenario.Resource("some-resource").APIPinnedVersion()))
				})
			})

			Context("when the resource is pinned by another version already", func() {
				BeforeEach(func() {
					found, err := scenario.Resource("some-resource").PinVersion(scenario.ResourceVersion("some-resource", atc.Version{"version": "v3"}).ID())
					Expect(found).To(BeTrue())
					Expect(err).ToNot(HaveOccurred())
				})

				It("switch the pin to given version", func() {
					Expect(scenario.Resource("some-resource").APIPinnedVersion()).To(Equal(atc.Version{"version": "v3"}))
					Expect(scenario.Resource("some-resource").CurrentPinnedVersion()).To(Equal(scenario.Resource("some-resource").APIPinnedVersion()))
				})
			})

			Context("when we set the pin comment on a resource", func() {
				BeforeEach(func() {
					err := scenario.Resource("some-resource").SetPinComment("foo")
					Expect(err).ToNot(HaveOccurred())
				})

				It("should set the pin comment", func() {
					Expect(scenario.Resource("some-resource").PinComment()).To(Equal("foo"))
				})
			})

			Context("when requesting schedule for version unpinning", func() {
				It("requests schedule on all jobs using the resource", func() {
					requestedSchedule := scenario.Job("job-using-resource").ScheduleRequestedTime()

					err := scenario.Resource("some-resource").UnpinVersion()
					Expect(err).ToNot(HaveOccurred())

					Expect(scenario.Job("job-using-resource").ScheduleRequestedTime()).Should(BeTemporally(">", requestedSchedule))
				})

				It("does not request schedule on jobs that do not use the resource", func() {
					requestedSchedule := scenario.Job("not-using-resource").ScheduleRequestedTime()

					err := scenario.Resource("some-resource").UnpinVersion()
					Expect(err).ToNot(HaveOccurred())

					Expect(scenario.Job("not-using-resource").ScheduleRequestedTime()).Should(BeTemporally("==", requestedSchedule))
				})
			})

			Context("when we unpin a resource to a version", func() {
				BeforeEach(func() {
					err := scenario.Resource("some-resource").UnpinVersion()
					Expect(err).ToNot(HaveOccurred())
				})

				It("sets the api pinned version to nil", func() {
					Expect(scenario.Resource("some-resource").APIPinnedVersion()).To(BeNil())
					Expect(scenario.Resource("some-resource").CurrentPinnedVersion()).To(BeNil())
				})

				It("unsets the pin comment", func() {
					Expect(scenario.Resource("some-resource").PinComment()).To(BeEmpty())
				})
			})
		})

		Context("when we pin a resource that is already pinned to a version (through the config)", func() {
			BeforeEach(func() {
				scenario.Run(
					builder.WithPipeline(atc.Config{
						Resources: atc.ResourceConfigs{
							{
								Name:    "some-resource",
								Type:    "some-base-resource-type",
								Source:  atc.Source{"some": "repository"},
								Version: atc.Version{"pinned": "version"},
							},
						},
					}),
					builder.WithResourceVersions(
						"some-resource",
						atc.Version{"version": "v1"},
						atc.Version{"version": "v2"},
						atc.Version{"version": "v3"},
					))
			})

			It("should fail to update the pinned version", func() {
				found, err := scenario.Resource("some-resource").PinVersion(scenario.ResourceVersion("some-resource", atc.Version{"version": "v1"}).ID())
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
