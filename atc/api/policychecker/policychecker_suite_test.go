package policychecker_test

import (
	"code.cloudfoundry.org/lager/lagertest"
	"github.com/concourse/concourse/atc/policy"
	"github.com/concourse/concourse/atc/policy/policyfakes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestAccessor(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "API PolicyChecker Suite")
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
})
