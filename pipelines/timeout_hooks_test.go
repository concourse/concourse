package pipelines_test

import (
	"github.com/concourse/testflight/gitserver"
	"github.com/concourse/testflight/guidserver"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("A pipeline containing a job with a timeout and hooks", func() {
	var (
		guidServer *guidserver.Server

		originGitServer  *gitserver.Server
		successGitServer *gitserver.Server
		failureGitServer *gitserver.Server
		ensureGitServer  *gitserver.Server
	)

	BeforeEach(func() {
		guidServer = guidserver.Start(guidServerRootfs, gardenClient)

		originGitServer = gitserver.Start(gitServerRootfs, gardenClient)
		successGitServer = gitserver.Start(gitServerRootfs, gardenClient)
		failureGitServer = gitserver.Start(gitServerRootfs, gardenClient)
		ensureGitServer = gitserver.Start(gitServerRootfs, gardenClient)

		configurePipeline(
			"-c", "fixtures/timeout_hooks.yml",
			"-v", "testflight-helper-image="+guidServerRootfs,
			"-v", "guid-server-curl-command="+guidServer.RegisterCommand(),
			"-v", "origin-git-server="+originGitServer.URI(),
			"-v", "success-git-server="+successGitServer.URI(),
			"-v", "failure-git-server="+failureGitServer.URI(),
			"-v", "ensure-git-server="+ensureGitServer.URI(),
		)
	})

	AfterEach(func() {
		guidServer.Stop()

		originGitServer.Stop()
		successGitServer.Stop()
		failureGitServer.Stop()
		ensureGitServer.Stop()
	})

	It("runs the failure and ensure hooks", func() {
		committedGuid := originGitServer.Commit()
		Eventually(guidServer.ReportingGuids).Should(ContainElement(committedGuid))

		masterSHA := originGitServer.RevParse("master")
		Expect(masterSHA).NotTo(BeEmpty())

		Eventually(func() string {
			return ensureGitServer.RevParse("ensure")
		}).Should(Equal(masterSHA))

		Eventually(func() string {
			return successGitServer.RevParse("success")
		}).Should(BeEmpty())

		Eventually(func() string {
			return failureGitServer.RevParse("failure")
		}).Should(Equal(masterSHA))
	})
})
