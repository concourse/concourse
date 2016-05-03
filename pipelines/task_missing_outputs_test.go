package pipelines_test

import (
	"github.com/concourse/testflight/gitserver"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("A task with no outputs declared", func() {
	var originGitServer *gitserver.Server

	BeforeEach(func() {
		originGitServer = gitserver.Start(client)

		configurePipeline(
			"-c", "fixtures/task-missing-outputs.yml",
			"-v", "origin-git-server="+originGitServer.URI(),
		)

		originGitServer.Commit()
	})

	AfterEach(func() {
		originGitServer.Stop()
	})

	It("doesn't mount its file system into the next task", func() {
		watch := flyWatch("missing-outputs-job")
		Expect(watch).To(gexec.Exit(2))

		Expect(watch).To(gbytes.Say("missing inputs: missing-outputs"))
	})
})
