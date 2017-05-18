package web_test

import (
	"fmt"
	"time"

	"github.com/concourse/testflight/gitserver"
	uuid "github.com/nu7hatch/gouuid"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/sclevine/agouti/matchers"
)

var _ = Describe("resource metadata", func() {
	var (
		originGitServer *gitserver.Server
	)

	BeforeEach(func() {
		originGitServer = gitserver.Start(client)
	})

	AfterEach(func() {
		originGitServer.Stop()
	})

	Context("when running build again on the same job", func() {
		It("prints resource metadata", func() {
			By("configuring pipeline")
			flyHelper.ConfigurePipeline(
				pipelineName,
				"-c", "fixtures/resource.yml",
				"-v", "origin-git-server="+originGitServer.URI(),
			)

			sha := originGitServer.Commit()

			By("triggering job")
			watch := flyHelper.TriggerJob(pipelineName, "simple")
			<-watch.Exited

			By("navigating to the build's page")
			Expect(page.Navigate(atcRoute(fmt.Sprintf("teams/%s/pipelines/%s/jobs/simple/builds/%d", teamName, pipelineName, 1)))).To(Succeed())
			Eventually(page.Find("h3"), 10*time.Second).Should(HaveText("some-git-resource"))

			By("clicking the header")
			Expect(page.Find(".header").Click()).To(Succeed()) // first resource

			By("seeing the metadata")
			Eventually(page.Find(".step-body"), 30*time.Second, 1*time.Second).Should(MatchText("commit.*" + sha))

			By("triggering the job again")
			watch = flyHelper.TriggerJob(pipelineName, "simple")
			<-watch.Exited

			By("navigating to the page")
			Expect(page.Navigate(atcRoute(fmt.Sprintf("teams/%s/pipelines/%s/jobs/simple/builds/%d", teamName, pipelineName, 2)))).To(Succeed())
			Eventually(page.Find("h3"), 10*time.Second).Should(HaveText("some-git-resource"))

			By("clicking the header")
			Expect(page.Find(".header").Click()).To(Succeed()) // first resource

			By("seeing the metadata")
			Eventually(page.Find(".step-body"), 30*time.Second, 1*time.Second).Should(MatchText("commit.*" + sha))
		})
	})

	Context("when running build on a another pipeline with the same resource config", func() {
		var (
			commitSHA           string
			anotherPipelineName string
		)

		BeforeEach(func() {
			guid, err := uuid.NewV4()
			Expect(err).NotTo(HaveOccurred())
			anotherPipelineName = "another-pipeline-" + guid.String()

			flyHelper.ConfigurePipeline(
				pipelineName,
				"-c", "fixtures/resource.yml",
				"-v", "origin-git-server="+originGitServer.URI(),
			)

			flyHelper.ConfigurePipeline(
				anotherPipelineName,
				"-c", "fixtures/resource.yml",
				"-v", "origin-git-server="+originGitServer.URI(),
			)

			commitSHA = originGitServer.Commit()
		})

		AfterEach(func() {
			flyHelper.DestroyPipeline(anotherPipelineName)
		})

		It("prints resource metadata", func() {
			By("triggering job")
			watch := flyHelper.TriggerJob(pipelineName, "simple")
			<-watch.Exited
			Expect(watch.ExitCode()).To(Equal(0))

			By("navigating to the build's page")
			Expect(page.Navigate(atcRoute(fmt.Sprintf("teams/%s/pipelines/%s/jobs/simple/builds/%d", teamName, pipelineName, 1)))).To(Succeed())
			Eventually(page.Find("h3"), 30*time.Second).Should(HaveText("some-git-resource"))

			By("clicking the header")
			Expect(page.Find(".header").Click()).To(Succeed()) // first resource

			By("seeing the metadata")
			Eventually(page.Find(".step-body"), 30*time.Second, 1*time.Second).Should(MatchText("commit.*" + commitSHA))

			By("triggering the job in another pipeline")
			watch = flyHelper.TriggerJob(anotherPipelineName, "simple")
			<-watch.Exited
			Expect(watch.ExitCode()).To(Equal(0))

			By("navigating to the other pipeline's build's page")
			Expect(page.Navigate(atcRoute(fmt.Sprintf("teams/%s/pipelines/%s/jobs/simple/builds/%d", teamName, anotherPipelineName, 1)))).To(Succeed())
			Eventually(page.Find("h3"), 30*time.Second).Should(HaveText("some-git-resource"))

			By("clicking the header")
			Expect(page.Find(".header").Click()).To(Succeed()) // first resource

			By("seeing the metadata")
			Eventually(page.Find(".step-body"), 30*time.Second, 1*time.Second).Should(MatchText("commit.*" + commitSHA))
		})
	})
})
