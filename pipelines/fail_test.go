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

	It("causes the build to fail", func() {
		watch := flyWatch("failing-job")
		Expect(watch).To(gbytes.Say("initializing"))
		Expect(watch).To(gbytes.Say("failed"))
		Expect(watch).To(gexec.Exit(1))
	})
})
