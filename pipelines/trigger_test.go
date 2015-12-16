package pipelines_test

import (
	"github.com/concourse/testflight/gitserver"
	"github.com/concourse/testflight/guidserver"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("A job with a git input with trigger: true", func() {
	var guidServer *guidserver.Server
	var originGitServer *gitserver.Server

	BeforeEach(func() {
		guidServer = guidserver.Start(guidServerRootfs, gardenClient)
		originGitServer = gitserver.Start(gitServerRootfs, gardenClient)

		configurePipeline(
			"-c", "fixtures/simple-trigger.yml",
			"-v", "testflight-helper-image="+guidServerRootfs,
			"-v", "guid-server-curl-command="+guidServer.RegisterCommand(),
			"-v", "origin-git-server="+originGitServer.URI(),
		)
	})

	AfterEach(func() {
		guidServer.Stop()
		originGitServer.Stop()
	})

	It("triggers when the repo changes", func() {
		By("building the initial commit")
		guid1 := originGitServer.Commit()
		Eventually(guidServer.ReportingGuids).Should(ContainElement(guid1))

		By("building another commit")
		guid2 := originGitServer.Commit()
		Eventually(guidServer.ReportingGuids).Should(ContainElement(guid2))
	})
})
