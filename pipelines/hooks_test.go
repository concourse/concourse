package git_pipeline_test

import (
	"github.com/concourse/testflight/gitserver"
	"github.com/concourse/testflight/guidserver"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("A pipeline containing jobs with hooks", func() {
	var (
		guidServer *guidserver.Server

		originGitServer        *gitserver.Server
		successGitServer       *gitserver.Server
		failureGitServer       *gitserver.Server
		noUpdateGitServer      *gitserver.Server
		ensureSuccessGitServer *gitserver.Server
		ensureFailureGitServer *gitserver.Server
	)

	BeforeEach(func() {
		guidServer = guidserver.Start(guidServerRootfs, gardenClient)

		originGitServer = gitserver.Start(gitServerRootfs, gardenClient)
		successGitServer = gitserver.Start(gitServerRootfs, gardenClient)
		failureGitServer = gitserver.Start(gitServerRootfs, gardenClient)
		noUpdateGitServer = gitserver.Start(gitServerRootfs, gardenClient)
		ensureSuccessGitServer = gitserver.Start(gitServerRootfs, gardenClient)
		ensureFailureGitServer = gitserver.Start(gitServerRootfs, gardenClient)

		configurePipeline(
			"-c", "fixtures/hooks.yml",
			"-v", "testflight-helper-image="+guidServerRootfs,
			"-v", "guid-server-curl-command="+guidServer.CurlCommand(),
			"-v", "origin-git-server="+originGitServer.URI(),
			"-v", "success-git-server="+successGitServer.URI(),
			"-v", "failure-git-server="+failureGitServer.URI(),
			"-v", "no-update-git-server="+noUpdateGitServer.URI(),
			"-v", "ensure-success-git-server="+ensureSuccessGitServer.URI(),
			"-v", "ensure-failure-git-server="+ensureFailureGitServer.URI(),
		)
	})

	AfterEach(func() {
		guidServer.Stop()

		originGitServer.Stop()
		successGitServer.Stop()
		failureGitServer.Stop()
		noUpdateGitServer.Stop()
		ensureSuccessGitServer.Stop()
		ensureFailureGitServer.Stop()
	})

	It("performs hooks under the right conditions", func() {
		By("executing the build when a commit is made")
		committedGuid := originGitServer.Commit()
		Eventually(guidServer.ReportingGuids).Should(ContainElement(committedGuid))

		masterSHA := originGitServer.RevParse("master")
		Ω(masterSHA).ShouldNot(BeEmpty())

		By("performing on_success outputs on success")
		Eventually(func() string {
			return successGitServer.RevParse("success")
		}).Should(Equal(masterSHA))

		By("performing on_failure steps on failure")
		Eventually(func() string {
			return failureGitServer.RevParse("failure")
		}).Should(Equal(masterSHA))

		By("not performing on_success steps on failure or on_failure steps on success")
		Consistently(func() string {
			return noUpdateGitServer.RevParse("no-update")
		}).Should(BeEmpty())

		By("performing ensure steps on success")
		Eventually(func() string {
			return ensureSuccessGitServer.RevParse("ensure-success")
		}).Should(Equal(masterSHA))

		By("peforming ensure steps on failure")
		Eventually(func() string {
			return ensureFailureGitServer.RevParse("ensure-failure")
		}).Should(Equal(masterSHA))
	})
})
