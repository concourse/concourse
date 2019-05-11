package resource_test

import (
	"testing"

	"github.com/concourse/concourse/atc/resource"
	"github.com/concourse/concourse/atc/worker/workerfakes"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var (
	fakeContainer *workerfakes.FakeContainer

	resourceForContainer resource.Resource
)

var _ = BeforeEach(func() {
	fakeContainer = new(workerfakes.FakeContainer)

	resourceForContainer = resource.NewResource(fakeContainer)
})

func TestResource(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Resource Suite")
}
