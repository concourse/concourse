package web_test

import (
	"fmt"

	"github.com/cloudfoundry/gunk/urljoiner"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/sclevine/agouti"

	"github.com/concourse/go-concourse/concourse"
	"github.com/concourse/testflight/helpers"

	"testing"
	"time"
)

var atcURL = helpers.AtcURL()

var pipelineName string

var client concourse.Client

func TestWeb(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Web Suite")
}

var agoutiDriver *agouti.WebDriver
var page *agouti.Page

var _ = BeforeSuite(func() {
	var err error
	client, err = helpers.ConcourseClient(atcURL)
	Expect(err).ToNot(HaveOccurred())

	// observed jobs taking ~1m30s, so set the timeout pretty high
	SetDefaultEventuallyTimeout(5 * time.Minute)

	// poll less frequently
	SetDefaultEventuallyPollingInterval(time.Second)

	agoutiDriver = agouti.PhantomJS(agouti.Debug)
	Expect(agoutiDriver.Start()).To(Succeed())

	pipelineName = fmt.Sprintf("test-pipeline-%d", GinkgoParallelNode())
})

var _ = AfterSuite(func() {
	Expect(agoutiDriver.Stop()).To(Succeed())
})

var _ = BeforeEach(func() {
	Skip("skipping until we can switch to ChromeDriver")

	_, err := client.DeletePipeline(pipelineName)
	Expect(err).ToNot(HaveOccurred())

	page, err = agoutiDriver.NewPage()
	Expect(err).NotTo(HaveOccurred())
	helpers.WebLogin(page, atcURL)
})

var _ = AfterEach(func() {
	Expect(page.Destroy()).To(Succeed())
})

func atcRoute(path string) string {
	return urljoiner.Join(atcURL, path)
}
