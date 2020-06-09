package exec_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"go.opentelemetry.io/otel/api/trace"
	"go.opentelemetry.io/otel/api/trace/testtrace"

	"testing"
)

func TestExec(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Exec Suite")
}

type testMetadata []string

func (m testMetadata) Env() []string { return m }

type testTraceProvider struct{}

func (ttp testTraceProvider) Tracer(name string) trace.Tracer {
	return testtrace.NewTracer()
}
