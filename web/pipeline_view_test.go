package web_test

import (
	"time"

	"github.com/concourse/atc"
	yaml "gopkg.in/yaml.v2"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/sclevine/agouti"
	. "github.com/sclevine/agouti/matchers"
)

var _ = Describe("PipelineView", func() {
	var (
		loadingTimeout time.Duration
	)

	BeforeEach(func() {
		loadingTimeout = 10 * time.Second

		config := atc.Config{
			Jobs: []atc.JobConfig{
				{Name: "some-job-name"},
			},
		}

		byteConfig, err := yaml.Marshal(config)
		Expect(err).NotTo(HaveOccurred())

		_, _, _, err = team.CreateOrUpdatePipelineConfig(pipelineName, "0", byteConfig)
		Expect(err).NotTo(HaveOccurred())
	})

	It("hides the legend after 10 seconds", func() {
		Expect(page.Navigate(atcURL)).To(Succeed())
		Eventually(page, loadingTimeout).Should(HaveURL(atcRoute("/")))

		Consistently(func() *agouti.Selection {
			return page.Find(".legend")
		}, loadingTimeout, 1*time.Second).Should(BeFound())
		Consistently(func() *agouti.Selection {
			return page.Find(".legend hidden")
		}, loadingTimeout, 1*time.Second).ShouldNot(BeFound())
		Eventually(page.Find(".legend.hidden"), loadingTimeout).Should(BeFound())
	})
})
