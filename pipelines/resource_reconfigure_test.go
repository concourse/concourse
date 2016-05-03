package pipelines_test

import (
	"github.com/concourse/testflight/gitserver"
	"github.com/concourse/testflight/guidserver"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Reconfiguring a resource", func() {
	var guidServer *guidserver.Server
	var originGitServer *gitserver.Server

	BeforeEach(func() {
		guidServer = guidserver.Start(client)
		originGitServer = gitserver.Start(client)
	})

	AfterEach(func() {
		guidServer.Stop()
		originGitServer.Stop()
	})

	It("creates a new check container with the updated configuration", func() {
		configurePipeline(
			"-c", "fixtures/simple-trigger.yml",
			"-v", "guid-server-curl-command="+guidServer.RegisterCommand(),
			"-v", "origin-git-server="+originGitServer.URI(),
		)

		guid1 := originGitServer.Commit()
		Eventually(guidServer.ReportingGuids).Should(ContainElement(guid1))

		reconfigurePipeline(
			"-c", "fixtures/simple-trigger-reconfigured.yml",
			"-v", "guid-server-curl-command="+guidServer.RegisterCommand(),
			"-v", "origin-git-server="+originGitServer.URI(),
		)

		guid2 := originGitServer.CommitOnBranch("other")
		Eventually(guidServer.ReportingGuids).Should(ContainElement(guid2))
	})
})
