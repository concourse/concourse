package web_test

import (
	"fmt"
	"time"

	"github.com/concourse/atc"
	uuid "github.com/nu7hatch/gouuid"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/sclevine/agouti/matchers"
)

var _ = Describe("PipelinePausing", func() {
	var (
		loadingTimeout time.Duration
	)

	Context("with a job in the configuration", func() {
		var anotherPipelineName string

		BeforeEach(func() {
			guid, err := uuid.NewV4()
			Expect(err).NotTo(HaveOccurred())
			anotherPipelineName = "another-pipeline-" + guid.String()

			_, _, _, err = team.CreateOrUpdatePipelineConfig(pipelineName, "0", atc.Config{
				Jobs: []atc.JobConfig{
					{Name: "some-job-name"},
				},
			})
			Expect(err).NotTo(HaveOccurred())
			_, err = team.UnpausePipeline(pipelineName)
			Expect(err).NotTo(HaveOccurred())

			_, err = team.DeletePipeline(anotherPipelineName)
			Expect(err).NotTo(HaveOccurred())

			_, _, _, err = team.CreateOrUpdatePipelineConfig(anotherPipelineName, "0", atc.Config{
				Jobs: []atc.JobConfig{
					{Name: "another-job-name"},
				},
			})
			Expect(err).NotTo(HaveOccurred())

			_, err = team.UnpausePipeline(anotherPipelineName)
			Expect(err).NotTo(HaveOccurred())

			loadingTimeout = 10 * time.Second
		})

		AfterEach(func() {
			_, err := team.DeletePipeline(anotherPipelineName)
			Expect(err).NotTo(HaveOccurred())
		})

		// homeLink := ".top-bar.test li:nth-of-type(2) a"
		navList := ".sidebar.test .team ul"

		It("can pause the pipelines", func() {
			Expect(page.Navigate(atcURL)).To(Succeed())
			Eventually(page, loadingTimeout).Should(HaveURL(atcRoute("/")))

			By("toggling the nav")
			Expect(page.Find(".sidebar-toggle.test").Click()).To(Succeed())

			By("clicking another pipeline")
			Eventually(page.All(navList).FindByLink(anotherPipelineName)).Should(BeFound())
			Expect(page.All(navList).FindByLink(anotherPipelineName).Click()).To(Succeed())
			Eventually(page, loadingTimeout).Should(HaveURL(atcRoute(fmt.Sprintf("/teams/%s/pipelines/%s", teamName, anotherPipelineName))))
			Eventually(page.Find(".pipeline-graph.test").Text, loadingTimeout).Should(ContainSubstring("another-job-name"))

			By("pausing another pipeline")
			spanXPath := fmt.Sprintf("//a[@href='/teams/%s/pipelines/%s']/preceding-sibling::span", teamName, anotherPipelineName)
			Eventually(page.All(navList).FindByXPath(spanXPath), loadingTimeout).Should(BeVisible())
			Expect(page.All(navList).FindByXPath(spanXPath + "[contains(@class, 'disabled')]")).To(BeFound())
			Expect(page.FindByXPath(spanXPath).Click()).To(Succeed())

			// top bar should show the pipeline is paused
			Eventually(page.Find(".top-bar.test.paused"), loadingTimeout).Should(BeFound())

			By("refreshing the page")
			page.Refresh()

			Eventually(page.Find(".top-bar.test.paused"), loadingTimeout).Should(BeFound())
			Expect(page.Find(".sidebar-toggle.test").Click()).To(Succeed())

			Eventually(page.All(navList).FindByXPath(spanXPath), loadingTimeout).Should(BeVisible())
			Expect(page.All(navList).FindByXPath(spanXPath + "[contains(@class, 'enabled')]")).To(BeFound())

			By("unpausing the pipeline")
			Expect(page.FindByXPath(spanXPath).Click()).To(Succeed())
			Eventually(page.All(navList).FindByXPath(spanXPath + "[contains(@class, 'disabled')]")).Should(BeFound())

			Eventually(page.Find(".top-bar.test.paused")).ShouldNot(BeFound())
		})
	})
})
