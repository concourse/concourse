package pipelines_test

import (
	"github.com/concourse/testflight/gitserver"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

var _ = Describe("A job with a complicated build plan", func() {
	var originGitServer *gitserver.Server

	BeforeEach(func() {
		originGitServer = gitserver.Start(client)

		flyHelper.ConfigurePipeline(
			pipelineName,
			"-c", "fixtures/matrix.yml",
			"-v", "origin-git-server="+originGitServer.URI(),
		)
	})

	AfterEach(func() {
		originGitServer.Stop()
	})

	It("executes the build plan correctly", func() {
		By("executing the build when a commit is made")
		committedGuid := originGitServer.Commit()

		By("propagating data between steps")
		watch := flyHelper.Watch(pipelineName, "fancy-build-matrix")
		Eventually(watch).Should(gbytes.Say("passing-unit-1/file passing-unit-2/file " + committedGuid))

		By("failing on aggregates if any branch failed")
		Eventually(watch).Should(gbytes.Say("failed " + committedGuid))
	})
})
