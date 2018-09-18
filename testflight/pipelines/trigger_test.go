package pipelines_test

import (
	"github.com/concourse/testflight/gitserver"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

var _ = Describe("A job with a git input with trigger: true", func() {
	var originGitServer *gitserver.Server

	BeforeEach(func() {
		originGitServer = gitserver.Start(client)

		flyHelper.ConfigurePipeline(
			pipelineName,
			"-c", "fixtures/simple-trigger.yml",
			"-v", "origin-git-server="+originGitServer.URI(),
		)
	})

	AfterEach(func() {
		originGitServer.Stop()
	})

	It("triggers when the repo changes", func() {
		By("building the initial commit")
		guid1 := originGitServer.Commit()
		watch := flyHelper.Watch(pipelineName, "some-passing-job")
		Eventually(watch).Should(gbytes.Say(guid1))

		By("building another commit")
		guid2 := originGitServer.Commit()
		watch = flyHelper.Watch(pipelineName, "some-passing-job", "2")
		Eventually(watch).Should(gbytes.Say(guid2))
	})
})
