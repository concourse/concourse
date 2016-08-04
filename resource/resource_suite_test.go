package resource_test

import (
	"testing"

	"code.cloudfoundry.org/clock/fakeclock"
	wfakes "github.com/concourse/atc/worker/workerfakes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/concourse/atc/resource"
)

var (
	workerClient  *wfakes.FakeClient
	fakeContainer *wfakes.FakeContainer
	fakeClock     *fakeclock.FakeClock

	resource Resource
)

var _ = BeforeEach(func() {
	workerClient = new(wfakes.FakeClient)

	fakeContainer = new(wfakes.FakeContainer)

	resource = NewResource(fakeContainer)
})

func TestResource(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Resource Suite")
}
