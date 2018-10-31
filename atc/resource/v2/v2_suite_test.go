package v2_test

import (
	"testing"

	"github.com/concourse/concourse/atc/db/dbfakes"
	res "github.com/concourse/concourse/atc/resource"
	"github.com/concourse/concourse/atc/resource/v2"
	"github.com/concourse/concourse/atc/worker/workerfakes"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var (
	workerClient  *workerfakes.FakeClient
	fakeContainer *workerfakes.FakeContainer

	resourceInfo       v2.ResourceInfo
	resource           res.Resource
	fakeResourceConfig *dbfakes.FakeResourceConfig
)

var _ = BeforeEach(func() {
	workerClient = new(workerfakes.FakeClient)
	fakeContainer = new(workerfakes.FakeContainer)
	fakeResourceConfig = new(dbfakes.FakeResourceConfig)

	resourceInfo = v2.ResourceInfo{
		Artifacts: v2.Artifacts{
			APIVersion: "2.0",
			Check:      "artifact check",
			Get:        "artifact get",
			Put:        "artifact put",
		},
	}

	resource = v2.NewResource(fakeContainer, resourceInfo, fakeResourceConfig)
})

func TestResource(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Resource V2 Suite")
}
