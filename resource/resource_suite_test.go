package resource_test

import (
	"testing"
	"time"

	wfakes "github.com/concourse/atc/worker/workerfakes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pivotal-golang/clock/fakeclock"

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
	fakeClock = fakeclock.NewFakeClock(time.Unix(123, 456))

	resource = NewResource(fakeContainer, fakeClock)
})

func TestResource(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Resource Suite")
}
