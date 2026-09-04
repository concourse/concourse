package exec_test

import (
	"testing"

	"code.cloudfoundry.org/lager/v3/lagertest"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/concourse/concourse/v8/atc"
	"github.com/concourse/concourse/v8/atc/exec"
	"github.com/concourse/concourse/v8/atc/policy"
	"github.com/concourse/concourse/v8/atc/policy/policyfakes"
	"github.com/concourse/concourse/v8/atc/util"
)

func init() {
	util.PanicSink = GinkgoWriter
}

func TestExec(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Exec Suite")
}

var (
	testLogger = lagertest.NewTestLogger("test")

	fakePolicyAgentFactory *policyfakes.FakeAgentFactory
)

var _ = BeforeSuite(func() {
	fakePolicyAgentFactory = new(policyfakes.FakeAgentFactory)
	fakePolicyAgentFactory.IsConfiguredReturns(true)
	fakePolicyAgentFactory.DescriptionReturns("fakeAgent")

	policy.RegisterAgent(fakePolicyAgentFactory)
})

var noopStepper exec.Stepper = func(atc.Plan) exec.Step {
	Fail("cannot create substep")
	return nil
}
