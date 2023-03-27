package exec_test

import (
	"testing"

	"code.cloudfoundry.org/lager/lagertest"
	. "github.com/onsi/ginkgo/v2"
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

var (
	testLogger = lagertest.NewTestLogger("test")

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
