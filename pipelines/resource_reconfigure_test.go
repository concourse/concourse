package pipelines_test

import (
	"github.com/concourse/testflight/gitserver"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

var _ = Describe("Reconfiguring a resource", func() {
	var originGitServer *gitserver.Server

	BeforeEach(func() {
		originGitServer = gitserver.Start(client)
	})

	AfterEach(func() {
		originGitServer.Stop()
	})

	It("creates a new check container with the updated configuration", func() {
		configurePipeline(
			"-c", "fixtures/simple-trigger.yml",
			"-v", "origin-git-server="+originGitServer.URI(),
		)

		guid1 := originGitServer.Commit()
		watch := flyWatch("some-passing-job", "1")
		Eventually(watch).Should(gbytes.Say(guid1))

		reconfigurePipeline(
			"-c", "fixtures/simple-trigger-reconfigured.yml",
			"-v", "origin-git-server="+originGitServer.URI(),
		)

		guid2 := originGitServer.CommitOnBranch("other")
		watch = flyWatch("some-passing-job", "2")
		Eventually(watch).Should(gbytes.Say(guid2))
	})
})
