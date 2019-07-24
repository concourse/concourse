package worker_test

import (
	"testing"

	"github.com/concourse/concourse/atc/worker/workerfakes"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var (
	fakeContainer *workerfakes.FakeContainer
)

var _ = BeforeEach(func() {
	fakeContainer = new(workerfakes.FakeContainer)
})

func TestResource(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Resource Suite")
}
