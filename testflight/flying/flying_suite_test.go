package flying_test

import (
	"fmt"
	"os"

	"github.com/concourse/concourse/go-concourse/concourse"
	"github.com/concourse/concourse/testflight/helpers"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"

	"github.com/nu7hatch/gouuid"
)

var (
	flyBin  string
	tmpHome string

	flyHelper       *helpers.FlyHelper
	concourseClient concourse.Client

	pipelineName string
)

var atcURL = helpers.AtcURL()
var username = helpers.AtcUsername()
var password = helpers.AtcPassword()
var targetedConcourse = "testflight"
var teamName = "testflight"

var _ = SynchronizedBeforeSuite(func() []byte {
	Eventually(helpers.ErrorPolling(atcURL)).ShouldNot(HaveOccurred())

	data, err := helpers.FirstNodeFlySetup(atcURL, targetedConcourse, teamName, username, password)
	Expect(err).NotTo(HaveOccurred())

	return data
}, func(data []byte) {
	var err error
	flyBin, tmpHome, err = helpers.AllNodeFlySetup(data)
	Expect(err).NotTo(HaveOccurred())

	flyHelper = &helpers.FlyHelper{Path: flyBin}
	concourseClient, err = helpers.AllNodeClientSetup(data)
	Expect(err).NotTo(HaveOccurred())
})

var _ = SynchronizedAfterSuite(func() {
}, func() {
	os.RemoveAll(tmpHome)
})

var _ = BeforeEach(func() {
	guid, err := uuid.NewV4()
	Expect(err).ToNot(HaveOccurred())

	pipelineName = fmt.Sprintf("test-pipeline-%d-%s", GinkgoParallelNode(), guid)
})

var _ = AfterEach(func() {
	flyHelper.DestroyPipeline(pipelineName)
})

func TestFlying(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Flying Suite")
}
