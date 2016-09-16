package web_test

import (
	"fmt"

	"code.cloudfoundry.org/urljoiner"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/sclevine/agouti"

	"github.com/concourse/go-concourse/concourse"
	"github.com/concourse/testflight/helpers"

	"testing"
)

var atcURL = helpers.AtcURL()

var pipelineName string
var teamName string

var client concourse.Client
var team concourse.Team

func TestWeb(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Web Suite")
}

var agoutiDriver *agouti.WebDriver
var page *agouti.Page

var _ = SynchronizedBeforeSuite(func() []byte {
	Eventually(helpers.ErrorPolling(atcURL)).ShouldNot(HaveOccurred())

	data, err := helpers.FirstNodeClientSetup(atcURL)
	Expect(err).NotTo(HaveOccurred())

	return data
}, func(data []byte) {
	var err error
	client, err = helpers.AllNodeClientSetup(data)
	Expect(err).NotTo(HaveOccurred())

	pipelineName = fmt.Sprintf("test-pipeline-%d", GinkgoParallelNode())
	teamName = "main"
	team = client.Team(teamName)

	agoutiDriver = helpers.AgoutiDriver()
	Expect(agoutiDriver.Start()).To(Succeed())
})

var _ = AfterSuite(func() {
	Expect(agoutiDriver.Stop()).To(Succeed())
})

var _ = BeforeEach(func() {
	_, err := team.DeletePipeline(pipelineName)
	Expect(err).ToNot(HaveOccurred())

	page, err = agoutiDriver.NewPage()
	Expect(err).NotTo(HaveOccurred())

	helpers.WebLogin(page, atcURL)
})

var _ = AfterEach(func() {
	Expect(page.Destroy()).To(Succeed())

	err := helpers.DeleteAllContainers(client, pipelineName)
	Expect(err).ToNot(HaveOccurred())

	_, err = team.DeletePipeline(pipelineName)
	Expect(err).ToNot(HaveOccurred())
})

func atcRoute(path string) string {
	return urljoiner.Join(atcURL, path)
}
