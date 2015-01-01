package resource_test

import (
	"testing"

	gfakes "github.com/cloudfoundry-incubator/garden/api/fakes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/concourse/atc/exec/resource"
)

var (
	gardenClient  *gfakes.FakeClient
	fakeContainer *gfakes.FakeContainer

	resource Resource
)

var _ = BeforeEach(func() {
	gardenClient = new(gfakes.FakeClient)

	fakeContainer = new(gfakes.FakeContainer)
	fakeContainer.HandleReturns("some-handle")

	resource = NewResource(fakeContainer, gardenClient)
})

func TestResource(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Resource Suite")
}
