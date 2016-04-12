package web_test

import (
	"time"

	"github.com/concourse/atc"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/sclevine/agouti/matchers"
)

var _ = Describe("PipelinePausing", func() {
	var (
		loadingTimeout time.Duration
	)

	Context("with a job in the configuration", func() {
		BeforeEach(func() {
			_, _, _, err := client.CreateOrUpdatePipelineConfig(pipelineName, "0", atc.Config{
				Jobs: []atc.JobConfig{
					{Name: "some-job-name"},
				},
			})
			Expect(err).NotTo(HaveOccurred())
			_, err = client.UnpausePipeline(pipelineName)
			Expect(err).NotTo(HaveOccurred())

			_, err = client.DeletePipeline("another-pipeline")
			Expect(err).NotTo(HaveOccurred())

			_, _, _, err = client.CreateOrUpdatePipelineConfig("another-pipeline", "0", atc.Config{
				Jobs: []atc.JobConfig{
					{Name: "another-job-name"},
				},
			})
			Expect(err).NotTo(HaveOccurred())

			_, err = client.UnpausePipeline("another-pipeline")
			Expect(err).NotTo(HaveOccurred())

			loadingTimeout = 10 * time.Second
		})

		AfterEach(func() {
			_, err := client.DeletePipeline("another-pipeline")
			Expect(err).NotTo(HaveOccurred())
		})

		homeLink := ".js-groups li:nth-of-type(2) a"
		defaultPipelineLink := ".js-pipelinesNav-list li:nth-of-type(1) a"
		anotherPipelineLink := ".js-pipelinesNav-list li:nth-of-type(2) a"
		anotherPipelineItem := ".js-pipelinesNav-list li:nth-of-type(2)"

		It("can pause the pipelines", func() {
			Expect(page.Navigate(atcURL)).To(Succeed())
			Eventually(page, loadingTimeout).Should(HaveURL(atcRoute("/")))

			Expect(page.Find(".js-pipelinesNav-toggle").Click()).To(Succeed())

			Expect(page.Find(defaultPipelineLink)).To(HaveText(pipelineName))
			Expect(page.Find(anotherPipelineLink)).To(HaveText("another-pipeline"))

			Expect(page.Find(anotherPipelineLink).Click()).To(Succeed())

			Eventually(page, loadingTimeout).Should(HaveURL(atcRoute("/pipelines/another-pipeline")))
			Expect(page.Find(homeLink).Click()).To(Succeed())
			Eventually(page, loadingTimeout).Should(HaveURL(atcRoute("/pipelines/another-pipeline")))

			Expect(page.Find(".js-pipelinesNav-toggle").Click()).To(Succeed())
			Eventually(page.Find(defaultPipelineLink), loadingTimeout).Should(HaveText(pipelineName))
			Eventually(page.Find("#pipeline").Text, loadingTimeout).Should(ContainSubstring("another-job-name"))

			Eventually(page.Find(anotherPipelineItem+" .js-pauseUnpause"), loadingTimeout).Should(BeVisible())
			Eventually(page.Find(anotherPipelineItem+" .js-pauseUnpause.disabled"), loadingTimeout).Should(BeFound())

			Expect(page.Find(anotherPipelineItem + " .js-pauseUnpause").Click()).To(Succeed())
			Eventually(page.Find(anotherPipelineItem+" .js-pauseUnpause.enabled"), loadingTimeout).Should(BeFound())

			// top bar should show the pipeline is paused
			Eventually(page.Find(".js-groups.paused"), loadingTimeout).Should(BeFound())

			page.Refresh()

			Eventually(page.Find(".js-groups.paused"), loadingTimeout).Should(BeFound())
			Expect(page.Find(".js-pipelinesNav-toggle").Click()).To(Succeed())
			Eventually(page.Find(anotherPipelineItem+" .js-pauseUnpause"), loadingTimeout).Should(BeVisible())
			Eventually(page.Find(anotherPipelineItem+" .js-pauseUnpause.enabled"), loadingTimeout).Should(BeFound())

			Expect(page.Find(anotherPipelineItem + " .js-pauseUnpause").Click()).To(Succeed())
			Eventually(page.Find(anotherPipelineItem+" .js-pauseUnpause.disabled"), loadingTimeout).Should(BeFound())

			Consistently(page.Find(".js-groups.paused")).ShouldNot(BeFound())

			page.Refresh()

			Expect(page.Find(".js-pipelinesNav-toggle").Click()).To(Succeed())
			Eventually(page.Find(anotherPipelineItem+" .js-pauseUnpause"), loadingTimeout).Should(BeVisible())
			Eventually(page.Find(anotherPipelineItem+" .js-pauseUnpause.disabled"), loadingTimeout).Should(BeFound())
		})
	})
})
