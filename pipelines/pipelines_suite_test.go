package pipelines_test

import (
	"fmt"
	"os"
	"strings"

	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"

	"github.com/concourse/go-concourse/concourse"
	"github.com/concourse/testflight/gitserver"
	"github.com/concourse/testflight/guidserver"
	"github.com/concourse/testflight/helpers"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"

	"github.com/nu7hatch/gouuid"
)

var (
	client concourse.Client
	team   concourse.Team

	flyHelper *helpers.FlyHelper

	pipelineName string

	tmpHome string
	logger  lager.Logger
)

var atcURL = helpers.AtcURL()

var _ = SynchronizedBeforeSuite(func() []byte {
	Eventually(helpers.ErrorPolling(atcURL)).ShouldNot(HaveOccurred())

	data, err := helpers.FirstNodeFlySetup(atcURL, helpers.TargetedConcourse)
	Expect(err).NotTo(HaveOccurred())

	client := helpers.ConcourseClient(atcURL)

	gitserver.Cleanup(client)
	guidserver.Cleanup(client)

	team = client.Team("main")

	pipelines, err := team.ListPipelines()
	Expect(err).ToNot(HaveOccurred())

	for _, pipeline := range pipelines {
		if strings.HasPrefix(pipeline.Name, "test-pipeline-") {
			_, err := team.DeletePipeline(pipeline.Name)
			Expect(err).ToNot(HaveOccurred())
		}
	}

	return data
}, func(data []byte) {
	var flyBinPath string
	var err error
	flyBinPath, tmpHome, err = helpers.AllNodeFlySetup(data)
	Expect(err).NotTo(HaveOccurred())

	flyHelper = &helpers.FlyHelper{Path: flyBinPath}

	client, err = helpers.AllNodeClientSetup(data)
	Expect(err).NotTo(HaveOccurred())

	team = client.Team("main")
	logger = lagertest.NewTestLogger("pipelines-test")
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

func TestGitPipeline(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Pipelines Suite")
}
