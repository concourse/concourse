/*
OpenTelemetry Reporter

The purpose of this reporter is to describe the ginkgo suite lifecycle in terms
of OpenTelemetry-compatible (https://opentelemetry.io/) spans in a distributed
tracing system.
*/
package tracing

import (
	"context"
	"os"
	"strconv"
	"strings"

	"github.com/onsi/ginkgo/config"
	"github.com/onsi/ginkgo/types"
	"go.opentelemetry.io/otel/api/core"
	"go.opentelemetry.io/otel/api/trace"
)

type OpenTelemetryReporter struct {
	tracer      trace.Tracer
	spanContext core.SpanContext
	ctx         context.Context
	currCtx     context.Context
}

type envSupplier struct{}

func (es *envSupplier) Get(key string) string {
	return os.Getenv(strings.ToUpper(key))
}

func (es *envSupplier) Set(key, value string) {
	os.Setenv(strings.ToUpper(key), value)
}

func NewOpenTelemetryReporter(
	tracer trace.Tracer,
) *OpenTelemetryReporter {
	ctx := trace.TraceContext{}.Extract(
		context.Background(),
		&envSupplier{},
	)
	return &OpenTelemetryReporter{
		tracer: tracer,
		ctx:    ctx,
	}
}

func (reporter *OpenTelemetryReporter) SpecSuiteWillBegin(
	config config.GinkgoConfigType,
	summary *types.SuiteSummary,
) {
	ctx, _ := reporter.tracer.Start(
		reporter.ctx,
		"suite",
	)
	reporter.ctx = ctx
	currCtx, _ := reporter.tracer.Start(ctx, "beforesuite")
	reporter.currCtx = currCtx
}

func (reporter *OpenTelemetryReporter) BeforeSuiteDidRun(
	setupSummary *types.SetupSummary,
) {
	beforeSuiteSpan := trace.SpanFromContext(reporter.currCtx)
	switch setupSummary.State {
	case types.SpecStateFailed:
		beforeSuiteSpan.SetAttributes(core.KeyValue{
			Key:   "state",
			Value: core.String("failed"),
		})
	case types.SpecStatePassed:
		beforeSuiteSpan.SetAttributes(core.KeyValue{
			Key:   "state",
			Value: core.String("passed"),
		})
	}
	beforeSuiteSpan.End()
	reporter.currCtx = reporter.ctx
}

func (reporter *OpenTelemetryReporter) SpecWillRun(specSummary *types.SpecSummary) {
	ctx, specSpan := reporter.tracer.Start(reporter.currCtx, "spec")
	for i, componentText := range specSummary.ComponentTexts {
		specSpan.SetAttributes(core.KeyValue{
			Key:   core.Key("component_texts[" + strconv.Itoa(i) + "]"),
			Value: core.String(componentText),
		})
	}
	reporter.currCtx = ctx
}

func (reporter *OpenTelemetryReporter) SpecDidComplete(specSummary *types.SpecSummary) {
	specSpan := trace.SpanFromContext(reporter.currCtx)
	specSpan.End()
	reporter.currCtx = reporter.ctx // TODO: test this
}

func (reporter *OpenTelemetryReporter) AfterSuiteDidRun(setupSummary *types.SetupSummary) {}

func (reporter *OpenTelemetryReporter) SpecSuiteDidEnd(summary *types.SuiteSummary) {
	rootSpan := trace.SpanFromContext(reporter.ctx)
	rootSpan.End()
}
