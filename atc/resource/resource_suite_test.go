package resource_test

import (
	"testing"

	"github.com/concourse/atc/resource"
	"github.com/concourse/atc/worker/workerfakes"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var (
	workerClient  *workerfakes.FakeClient
	fakeContainer *workerfakes.FakeContainer

	resourceForContainer resource.Resource
)

var _ = BeforeEach(func() {
	workerClient = new(workerfakes.FakeClient)

	fakeContainer = new(workerfakes.FakeContainer)

	resourceForContainer = resource.NewResourceForContainer(fakeContainer)
})

func TestResource(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Resource Suite")
}
