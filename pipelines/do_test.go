package pipelines_test

import (
	"github.com/concourse/testflight/gitserver"
	"github.com/concourse/testflight/guidserver"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("A pipeline containing a do", func() {
	var (
		guidServer *guidserver.Server

		originGitServer *gitserver.Server
		doGitServer     *gitserver.Server
	)

	BeforeEach(func() {
		guidServer = guidserver.Start(guidServerRootfs, gardenClient)

		originGitServer = gitserver.Start(gitServerRootfs, gardenClient)
		doGitServer = gitserver.Start(gitServerRootfs, gardenClient)

		configurePipeline(
			"-c", "fixtures/do.yml",
			"-v", "testflight-helper-image="+guidServerRootfs,
			"-v", "guid-server-curl-command="+guidServer.CurlCommand(),
			"-v", "origin-git-server="+originGitServer.URI(),
			"-v", "do-git-server="+doGitServer.URI(),
		)
	})

	AfterEach(func() {
		guidServer.Stop()

		originGitServer.Stop()
		doGitServer.Stop()
	})

	It("performs the do steps", func() {
		By("executing the build when a commit is made")
		committedGuid := originGitServer.Commit()
		Eventually(guidServer.ReportingGuids).Should(ContainElement(committedGuid))

		masterSHA := originGitServer.RevParse("master")
		Expect(masterSHA).NotTo(BeEmpty())

		By("running the first step")
		Eventually(func() string {
			return doGitServer.RevParse("do-1")
		}).Should(Equal(masterSHA))

		By("running the second step")
		Eventually(func() string {
			return doGitServer.RevParse("do-2")
		}).Should(Equal(masterSHA))

		By("running the third step")
		Eventually(func() string {
			return doGitServer.RevParse("do-3")
		}).Should(Equal(masterSHA))
	})
})
