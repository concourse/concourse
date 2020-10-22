package exec_test

import (
	"testing"

	"code.cloudfoundry.org/lager/lagertest"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"go.opentelemetry.io/otel/api/trace"
	"go.opentelemetry.io/otel/api/trace/tracetest"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/exec"
	"github.com/concourse/concourse/atc/policy"
	"github.com/concourse/concourse/atc/policy/policyfakes"
)

func TestExec(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Exec Suite")
}

type testMetadata []string

func (m testMetadata) Env() []string { return m }

type testTraceProvider struct{}

func (ttp testTraceProvider) Tracer(name string) trace.Tracer {
	return tracetest.NewProvider().Tracer("concourse")
}

var (
	testLogger = lagertest.NewTestLogger("test")

	fakePolicyAgent        *policyfakes.FakeAgent
	fakePolicyAgentFactory *policyfakes.FakeAgentFactory
)

var _ = BeforeSuite(func() {
	fakePolicyAgentFactory = new(policyfakes.FakeAgentFactory)
	fakePolicyAgentFactory.IsConfiguredReturns(true)
	fakePolicyAgentFactory.DescriptionReturns("fakeAgent")

	policy.RegisterAgent(fakePolicyAgentFactory)

	atc.EnablePipelineInstances = true
})

var noopStepper exec.Stepper = func(atc.Plan) exec.Step {
	Fail("cannot create substep")
	return nil
}
