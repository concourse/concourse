package resource_test

import (
	"testing"

	"github.com/concourse/concourse/v5/atc/resource"
	"github.com/concourse/concourse/v5/atc/worker/workerfakes"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var (
	fakeContainer *workerfakes.FakeContainer

	resourceForContainer resource.Resource
)

var _ = BeforeEach(func() {
	fakeContainer = new(workerfakes.FakeContainer)

	resourceFactory := resource.NewResourceFactory()
	resourceForContainer = resourceFactory.NewResourceForContainer(fakeContainer)
})

func TestResource(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Resource Suite")
}
