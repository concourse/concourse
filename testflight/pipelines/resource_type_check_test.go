package pipelines_test

import (
	"fmt"

	"github.com/concourse/concourse/testflight/gitserver"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
)

// XXX: make it better
var _ = Describe("Resource-types checks", func() {

	Context("Updating resource types", func() {
		var originGitServer *gitserver.Server

		BeforeEach(func() {
			originGitServer = gitserver.Start(client)
			originGitServer.CommitResource()
			originGitServer.CommitFileToBranch("initial", "initial", "trigger")

			flyHelper.ConfigurePipeline(
				pipelineName,
				"-c", "fixtures/resource-types.yml",
				"-v", "origin-git-server="+originGitServer.URI(),
				"-y", "privileged=false",
			)
		})

		AfterEach(func() {
			originGitServer.Stop()
		})

		It("uses updated resource type", func() {
			By("watching for first resource-imgur")
			watch := flyHelper.Watch(pipelineName, "resource-imgur", "1")
			Expect(watch).To(gbytes.Say("fetched from custom resource"))
			Expect(watch).To(gexec.Exit(0))

			originGitServer.CommitFileToBranch("new-contents", "rootfs/some-file", "master")
			originGitServer.CommitFileToBranch("new-version", "rootfs/version", "master")

			buildNum := 2
			Eventually(func() *gexec.Session {
				By("watching for resource-imgur with updated resource type")
				originGitServer.CommitFileToBranch(fmt.Sprintf("trigger %d", buildNum), "trigger", "trigger")

				watch = flyHelper.Watch(pipelineName, "resource-imgur", fmt.Sprintf("%d", buildNum))
				Expect(watch).To(gexec.Exit(0))
				buildNum += 1
				return watch
			}, "10s").Should(gbytes.Say("new-contents"))
		})
	})

	Context("check-resource-type", func() {
		var originGitServer *gitserver.Server

		BeforeEach(func() {
			originGitServer = gitserver.Start(client)
			originGitServer.CommitResource()
			originGitServer.CommitFileToBranch("initial", "initial", "trigger")

			flyHelper.ConfigurePipeline(
				pipelineName,
				"-c", "fixtures/resource-types.yml",
				"-v", "origin-git-server="+originGitServer.URI(),
				"-y", "privileged=false",
			)
		})

		AfterEach(func() {
			originGitServer.Stop()
		})

		It("can check the resource-type", func() {
			watch := flyHelper.CheckResourceType("-r", pipelineName+"/custom-resource-type")

			Eventually(watch).Should(gbytes.Say("checked 'custom-resource-type'"))
			Eventually(watch).Should(gexec.Exit(0))
		})

		It("reports that resource-type is not found if it doesn't exist", func() {
			watch := flyHelper.CheckResourceType("-r", pipelineName+"/nonexistent-resource-type")

			Eventually(watch.Err).Should(gbytes.Say("resource-type 'nonexistent-resource-type' not found"))
			Eventually(watch).Should(gexec.Exit(1))
		})

		It("fails when resource-type check fails", func() {
			watch := flyHelper.CheckResourceType("-r", pipelineName+"/failing-custom-resource-type")

			Eventually(watch.Err).Should(gbytes.Say("check failed"))
			Eventually(watch).Should(gexec.Exit(1))
		})
	})

})
