package web_test

import (
	"fmt"
	"os"

	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"
	"code.cloudfoundry.org/urljoiner"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/sclevine/agouti"

	"github.com/concourse/go-concourse/concourse"
	"github.com/concourse/testflight/helpers"

	"testing"
)

var (
	atcURL       = helpers.AtcURL()
	pipelineName string
	teamName     string
	client       concourse.Client
	team         concourse.Team
	logger       lager.Logger

	flyHelper *helpers.FlyHelper
	tmpHome   string
)

func TestWeb(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Web Suite")
}

var agoutiDriver *agouti.WebDriver
var page *agouti.Page

var _ = SynchronizedBeforeSuite(func() []byte {
	Eventually(helpers.ErrorPolling(atcURL)).ShouldNot(HaveOccurred())

	data, err := helpers.FirstNodeFlySetup(atcURL, helpers.TargetedConcourse)
	Expect(err).NotTo(HaveOccurred())

	return data
}, func(data []byte) {
	agoutiDriver = helpers.AgoutiDriver()
	Expect(agoutiDriver.Start()).To(Succeed())

	var flyBinPath string
	var err error
	flyBinPath, tmpHome, err = helpers.AllNodeFlySetup(data)
	Expect(err).NotTo(HaveOccurred())

	flyHelper = &helpers.FlyHelper{Path: flyBinPath}

	client, err = helpers.AllNodeClientSetup(data)
	Expect(err).NotTo(HaveOccurred())

	pipelineName = fmt.Sprintf("test-pipeline-%d", GinkgoParallelNode())
	teamName = "main"
	team = client.Team(teamName)
})

var _ = SynchronizedAfterSuite(func() {
}, func() {
	Expect(agoutiDriver.Stop()).To(Succeed())
	os.RemoveAll(tmpHome)
})

var _ = BeforeEach(func() {
	_, err := team.DeletePipeline(pipelineName)
	Expect(err).ToNot(HaveOccurred())

	page, err = agoutiDriver.NewPage()
	Expect(err).NotTo(HaveOccurred())

	Expect(helpers.WebLogin(page, atcURL)).To(Succeed())
	logger = lagertest.NewTestLogger("web-test")
})

var _ = AfterEach(func() {
	Expect(page.Destroy()).To(Succeed())

	flyHelper.DestroyPipeline(pipelineName)
})

func atcRoute(path string) string {
	return urljoiner.Join(atcURL, path)
}
