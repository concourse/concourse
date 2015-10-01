package git_pipeline_test

import (
	"github.com/concourse/testflight/gitserver"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("A job with a try step", func() {
	var originGitServer *gitserver.Server

	BeforeEach(func() {
		originGitServer = gitserver.Start(gitServerRootfs, gardenClient)

		configurePipeline(
			"-c", "fixtures/try.yml",
			"-v", "origin-git-server="+originGitServer.URI(),
		)

		originGitServer.Commit()
	})

	AfterEach(func() {
		originGitServer.Stop()
	})

	It("proceeds through the plan even if the step fails", func() {
		watch := flyWatch("try-job")
		Expect(watch).To(gbytes.Say("initializing"))
		Expect(watch).To(gbytes.Say("passing-task succeeded"))
		Expect(watch).To(gexec.Exit(0))
	})
})
