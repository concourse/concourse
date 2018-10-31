package v1_test

import (
	"testing"

	"github.com/concourse/concourse/atc/resource/v1"
	"github.com/concourse/concourse/atc/worker/workerfakes"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var (
	workerClient  *workerfakes.FakeClient
	fakeContainer *workerfakes.FakeContainer

	resourceForContainer v1.Resource
)

var _ = BeforeEach(func() {
	workerClient = new(workerfakes.FakeClient)

	fakeContainer = new(workerfakes.FakeContainer)

	resourceForContainer = v1.Resource{
		Container: fakeContainer,
	}
})

func TestResource(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Resource V1 Suite")
}
