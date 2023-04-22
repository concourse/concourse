package db_test

import (
	"context"
	"time"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/db/dbtest"
	"github.com/concourse/concourse/tracing"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.opentelemetry.io/otel/oteltest"
	"go.opentelemetry.io/otel/trace"
)

var _ = Describe("Prototype", func() {
	var pipeline db.Pipeline

	BeforeEach(func() {
		var (
			created bool
			err     error
		)

		pipeline, created, err = defaultTeam.SavePipeline(
			atc.PipelineRef{Name: "pipeline-with-types"},
			atc.Config{
				Prototypes: atc.Prototypes{
					{
						Name:     "some-type",
						Type:     "registry-image",
						Source:   atc.Source{"some": "repository"},
						Defaults: atc.Source{"some-default-k1": "some-default-v1"},
					},
					{
						Name:       "some-other-type",
						Type:       "some-type",
						Privileged: true,
						Source:     atc.Source{"some": "other-repository"},
					},
					{
						Name:   "some-type-with-params",
						Type:   "s3",
						Source: atc.Source{"some": "repository"},
						Params: atc.Params{"unpack": "true"},
					},
					{
						Name:       "some-type-with-custom-check",
						Type:       "registry-image",
						Source:     atc.Source{"some": "repository"},
						CheckEvery: &atc.CheckEvery{Interval: 10 * time.Millisecond},
					},
				},
			},
			0,
			false,
		)
		Expect(err).ToNot(HaveOccurred())
		Expect(created).To(BeTrue())
	})

	Describe("(Pipeline).Prototypes", func() {
		var prototypes db.Prototypes

		JustBeforeEach(func() {
			var err error
			prototypes, err = pipeline.Prototypes()
			Expect(err).ToNot(HaveOccurred())
		})

		It("returns the prototypes", func() {
			Expect(prototypes.Configs()).To(ConsistOf(
				atc.Prototypes{
					{
						Name:     "some-type",
						Type:     "registry-image",
						Source:   atc.Source{"some": "repository"},
						Defaults: atc.Source{"some-default-k1": "some-default-v1"},
					},
					{
						Name:       "some-other-type",
						Type:       "some-type",
						Privileged: true,
						Source:     atc.Source{"some": "other-repository"},
					},
					{
						Name:   "some-type-with-params",
						Type:   "s3",
						Source: atc.Source{"some": "repository"},
						Params: atc.Params{"unpack": "true"},
					},
					{
						Name:       "some-type-with-custom-check",
						Type:       "registry-image",
						Source:     atc.Source{"some": "repository"},
						CheckEvery: &atc.CheckEvery{Interval: 10 * time.Millisecond},
					},
				},
			))

			for _, prototype := range prototypes {
				Expect(prototype.Version()).To(BeNil())
			}
		})

		Context("when a prototype becomes inactive", func() {
			BeforeEach(func() {
				var (
					created bool
					err     error
				)

				pipeline, created, err = defaultTeam.SavePipeline(
					atc.PipelineRef{Name: "pipeline-with-types"},
					atc.Config{
						Prototypes: atc.Prototypes{
							{
								Name:   "some-type",
								Type:   "registry-image",
								Source: atc.Source{"some": "repository"},
							},
						},
					},
					pipeline.ConfigVersion(),
					false,
				)
				Expect(err).ToNot(HaveOccurred())
				Expect(created).To(BeFalse())
			})

			It("does not return inactive prototypes", func() {
				Expect(prototypes).To(HaveLen(1))
				Expect(prototypes[0].Name()).To(Equal("some-type"))
			})
		})
	})

	Describe("Prototype version", func() {
		var (
			scenario *dbtest.Scenario
			scope    db.ResourceConfigScope
		)

		BeforeEach(func() {
			scenario = dbtest.Setup(
				builder.WithPipeline(atc.Config{
					Prototypes: atc.Prototypes{
						{
							Name:   "some-type",
							Type:   "some-base-resource-type",
							Source: atc.Source{"some": "repository"},
						},
					},
				}),
			)
			Expect(scenario.Prototype("some-type").Version()).To(BeNil())

			scenario.Run(builder.WithPrototypeVersions("some-type"))

			prototypeResourceConfig, err := resourceConfigFactory.FindOrCreateResourceConfig(
				scenario.Prototype("some-type").Type(),
				scenario.Prototype("some-type").Source(),
				nil,
			)
			Expect(err).ToNot(HaveOccurred())

			scope, err = prototypeResourceConfig.FindOrCreateScope(nil)
			Expect(err).ToNot(HaveOccurred())
		})

		JustBeforeEach(func() {
			ok, err := scenario.Prototype("some-type").Reload()
			Expect(err).ToNot(HaveOccurred())
			Expect(ok).To(BeTrue())
		})

		It("creates a shared scope for the prototype", func() {
			Expect(scope.ResourceID()).To(BeNil())
			Expect(scope.ResourceConfig()).ToNot(BeNil())
		})

		It("returns the resource config scope id", func() {
			Expect(scenario.Prototype("some-type").ResourceConfigScopeID()).To(Equal(scope.ID()))
		})

		Context("when the prototype has proper versions", func() {
			BeforeEach(func() {
				scenario.Run(builder.WithPrototypeVersions("some-type",
					atc.Version{"version": "1"},
					atc.Version{"version": "2"},
				))
			})

			It("returns the version", func() {
				Expect(scenario.Prototype("some-type").Version()).To(Equal(atc.Version{"version": "2"}))
			})
		})
	})

	Describe("CheckPlan", func() {
		var prototype db.Prototype
		var resourceTypes db.ResourceTypes

		BeforeEach(func() {
			var err error
			var found bool
			prototype, found, err = pipeline.Prototype("some-type")
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())

			resourceTypes, err = pipeline.ResourceTypes()
			Expect(err).ToNot(HaveOccurred())
		})

		It("returns a plan which will update the prototype", func() {
			defaults := atc.Source{"sdk": "sdv"}
			Expect(prototype.CheckPlan(atc.NewPlanFactory(0), resourceTypes.Deserialize(), atc.Version{"some": "version"}, atc.CheckEvery{Interval: 1 * time.Hour}, defaults, false, false)).To(Equal(
				atc.Plan{
					ID: atc.PlanID("1"),
					Check: &atc.CheckPlan{
						Name:   prototype.Name(),
						Type:   prototype.Type(),
						Source: defaults.Merge(prototype.Source()),
						Tags:   prototype.Tags(),

						FromVersion: atc.Version{"some": "version"},
						Interval: atc.CheckEvery{
							Interval: 1 * time.Hour,
						},

						TypeImage: atc.TypeImage{
							BaseType: "registry-image",
						},

						Prototype: prototype.Name(),
					},
				}))
		})
	})

	Describe("CreateBuild", func() {
		var prototype db.Prototype
		var ctx context.Context
		var manuallyTriggered bool
		var plan atc.Plan

		var build db.Build
		var created bool

		BeforeEach(func() {
			ctx = context.TODO()

			var found bool
			var err error
			prototype, found, err = pipeline.Prototype("some-type")
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())

			manuallyTriggered = false
			plan = atc.Plan{
				ID: "some-plan",
				Check: &atc.CheckPlan{
					Name: "wreck",
				},
			}
		})

		JustBeforeEach(func() {
			var err error
			build, created, err = prototype.CreateBuild(ctx, manuallyTriggered, plan)
			Expect(err).ToNot(HaveOccurred())
		})

		It("creates a started build for a prototype", func() {
			Expect(created).To(BeTrue())
			Expect(build).ToNot(BeNil())
			Expect(build.Name()).To(Equal(db.CheckBuildName))
			Expect(build.PipelineID()).To(Equal(prototype.PipelineID()))
			Expect(build.TeamID()).To(Equal(prototype.TeamID()))
			Expect(build.IsManuallyTriggered()).To(BeFalse())
			Expect(build.Status()).To(Equal(db.BuildStatusStarted))
			Expect(build.PrivatePlan()).To(Equal(plan))
		})

		Context("when tracing is configured", func() {
			var span trace.Span

			BeforeEach(func() {
				tracing.ConfigureTraceProvider(oteltest.NewTracerProvider())

				ctx, span = tracing.StartSpan(context.Background(), "fake-operation", nil)
			})

			AfterEach(func() {
				tracing.Configured = false
			})

			It("propagates span context", func() {
				traceID := span.SpanContext().TraceID().String()
				buildContext := build.SpanContext()
				traceParent := buildContext.Get("traceparent")
				Expect(traceParent).To(ContainSubstring(traceID))
			})
		})
	})
})
