package git_pipeline_test

import (
	"github.com/concourse/testflight/gitserver"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("A job with a task that always fails", func() {
	var originGitServer *gitserver.Server

	BeforeEach(func() {
		originGitServer = gitserver.Start(gitServerRootfs, gardenClient)

		configurePipeline(
			"-c", "fixtures/fail.yml",
			"-v", "origin-git-server="+originGitServer.URI(),
		)

		originGitServer.Commit()
	})

	AfterEach(func() {
		originGitServer.Stop()
	})

	It("proceeds through the plan even if the step fails", func() {
		watch := flyWatch("failing-job")
		Ω(watch).Should(gbytes.Say("initializing"))
		Ω(watch).Should(gbytes.Say("failed"))
		Ω(watch).Should(gexec.Exit(1))
	})
})
