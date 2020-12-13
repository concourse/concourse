package exec_test

import (
	"context"
	"testing"

	"code.cloudfoundry.org/lager/lagerctx"
	"code.cloudfoundry.org/lager/lagertest"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/exec"
	"github.com/concourse/concourse/atc/policy"
	"github.com/concourse/concourse/atc/policy/policyfakes"
	"github.com/concourse/concourse/atc/util"
)

func init() {
	util.PanicSink = GinkgoWriter
}

func TestExec(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Exec Suite")
}

type testMetadata []string

func (m testMetadata) Env() []string { return m }

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

// hack to get context equality checking working when checking that the span
// context is propagated
func rewrapLogger(ctx context.Context) context.Context {
	return lagerctx.NewContext(ctx, lagerctx.FromContext(ctx))
}
