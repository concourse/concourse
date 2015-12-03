package acceptance_test

import (
	"fmt"

	"github.com/cloudfoundry/gunk/urljoiner"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/sclevine/agouti"

	"github.com/concourse/go-concourse/concourse"

	"testing"
	"time"
)

const atcURL = "http://10.244.15.2:8080"

var pipelineName string

var client concourse.Client

func TestWeb(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Web Suite")
}

var agoutiDriver *agouti.WebDriver

var _ = BeforeSuite(func() {
	conn, err := concourse.NewConnection(atcURL, nil)
	Expect(err).ToNot(HaveOccurred())

	client = concourse.NewClient(conn)

	// observed jobs taking ~1m30s, so set the timeout pretty high
	SetDefaultEventuallyTimeout(5 * time.Minute)

	// poll less frequently
	SetDefaultEventuallyPollingInterval(time.Second)

	agoutiDriver = agouti.PhantomJS()
	Expect(agoutiDriver.Start()).To(Succeed())

	pipelineName = fmt.Sprintf("test-pipeline-%d", GinkgoParallelNode())
})

var _ = AfterSuite(func() {
	Expect(agoutiDriver.Stop()).To(Succeed())
})

var _ = BeforeEach(func() {
	_, err := client.DeletePipeline(pipelineName)
	Expect(err).ToNot(HaveOccurred())
})

func Screenshot(page *agouti.Page) {
	page.Screenshot("/tmp/screenshot.png")
}

func atcRoute(path string) string {
	return urljoiner.Join(atcURL, path)
}
