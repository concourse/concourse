package pipelines_test

import (
	"github.com/concourse/testflight/gitserver"
	"github.com/concourse/testflight/guidserver"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("A job with a complicated build plan", func() {
	var guidServer *guidserver.Server
	var originGitServer *gitserver.Server

	BeforeEach(func() {
		guidServer = guidserver.Start(guidServerRootfs, gardenClient)
		originGitServer = gitserver.Start(gitServerRootfs, gardenClient)

		configurePipeline(
			"-c", "fixtures/matrix.yml",
			"-v", "testflight-helper-image="+guidServerRootfs,
			"-v", "guid-server-curl-command="+guidServer.CurlCommand(),
			"-v", "origin-git-server="+originGitServer.URI(),
		)
	})

	AfterEach(func() {
		guidServer.Stop()
		originGitServer.Stop()
	})

	It("executes the build plan correctly", func() {
		By("executing the build when a commit is made")
		committedGuid := originGitServer.Commit()

		By("propagating data between steps")
		Eventually(guidServer.ReportingGuids).Should(ContainElement("passing-unit-1/file passing-unit-2/file " + committedGuid))

		By("failing on aggregates if any branch failed")
		Eventually(guidServer.ReportingGuids).Should(ContainElement("failed " + committedGuid))
	})
})
