package tracing_test

import (
	"github.com/concourse/concourse/tracing"
	"github.com/onsi/ginkgo/config"
	"github.com/onsi/ginkgo/types"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"go.opentelemetry.io/otel/api/core"
	"go.opentelemetry.io/otel/api/trace/testtrace"
)

var _ = Describe("OpenTelemetryReporter", func() {
	var (
		reporter *tracing.OpenTelemetryReporter
		tracer   *testtrace.Tracer

		ginkgoConfig config.GinkgoConfigType
		suite        *types.SuiteSummary
	)

	BeforeEach(func() {
		tracer = testtrace.NewTracer()
		reporter = tracing.NewOpenTelemetryReporter(tracer)
	})

	Describe("SpecSuiteWillBegin", func() {
		BeforeEach(func() {
			suite = &types.SuiteSummary{
				SuiteDescription:           "A Sweet Suite",
				NumberOfTotalSpecs:         10,
				NumberOfSpecsThatWillBeRun: 8,
			}

			ginkgoConfig = config.GinkgoConfigType{
				RandomSeed:        1138,
				RandomizeAllSpecs: true,
			}
		})

		Context("when a suite begins", func() {
			BeforeEach(func() {
				reporter.SpecSuiteWillBegin(ginkgoConfig, suite)
			})

			It("should create a root span for the suite", func() {
				spans := tracer.Spans()
				Ω(len(spans)).Should(BeNumerically(">=", 1))
				Ω(spans[0].Name()).Should(
					Equal("suite"),
					"wrong span name",
				)
			})

			It("should create a child span for the BeforeSuite", func() {
				spans := tracer.Spans()
				Ω(spans[1].Name()).Should(
					Equal("beforesuite"),
					"wrong span name",
				)
			})
		})
	})

	Describe("BeforeSuiteDidRun", func() {
		It("ends the BeforeSuite span", func() {
			reporter.SpecSuiteWillBegin(ginkgoConfig, suite)

			reporter.BeforeSuiteDidRun(&types.SetupSummary{})

			_, ended := tracer.Spans()[1].EndTime()
			Ω(ended).Should(BeTrue(), "span should be done")
		})

		It("marks the span as succeeded if the BeforeSuite passes", func() {
			reporter.SpecSuiteWillBegin(ginkgoConfig, suite)

			reporter.BeforeSuiteDidRun(&types.SetupSummary{
				State: types.SpecStatePassed,
			})

			Ω(tracer.Spans()[1].Attributes()["state"]).Should(
				Equal(core.String("passed")),
			)
		})

		It("marks the span as failed if the BeforeSuite fails", func() {
			reporter.SpecSuiteWillBegin(ginkgoConfig, suite)

			reporter.BeforeSuiteDidRun(&types.SetupSummary{
				State: types.SpecStateFailed,
			})

			Ω(tracer.Spans()[1].Attributes()["state"]).Should(
				Equal(core.String("failed")),
			)
		})
	})

	Describe("SpecWillRun", func() {
		It("starts a span for the spec", func() {
			reporter.SpecSuiteWillBegin(ginkgoConfig, suite)

			reporter.SpecWillRun(&types.SpecSummary{})

			spans := tracer.Spans()
			Ω(spans).Should(HaveLen(3))
			Ω(spans[2].Name()).Should(
				Equal("spec"),
				"wrong span name",
			)
		})

		It("annotates the span with component texts", func() {
			reporter.SpecSuiteWillBegin(ginkgoConfig, suite)

			reporter.SpecWillRun(&types.SpecSummary{
				ComponentTexts: []string{
					"some context",
					"more context",
					"spec name",
				},
			})

			specSpanAttrs := tracer.Spans()[2].Attributes()
			Ω(specSpanAttrs["component_texts[0]"]).Should(
				Equal(core.String("some context")),
			)
			Ω(specSpanAttrs["component_texts[1]"]).Should(
				Equal(core.String("more context")),
			)
			Ω(specSpanAttrs["component_texts[2]"]).Should(
				Equal(core.String("spec name")),
			)
		})

		// TODO what about pending/skipped tests?
	})

	Describe("SpecDidComplete", func() {
		It("ends the spec span", func() {
			reporter.SpecSuiteWillBegin(ginkgoConfig, suite)
			spec := &types.SpecSummary{}
			reporter.SpecWillRun(spec)

			reporter.SpecDidComplete(spec)

			_, ended := tracer.Spans()[2].EndTime()
			Ω(ended).Should(BeTrue(), "span should be done")
		})
	})
})
