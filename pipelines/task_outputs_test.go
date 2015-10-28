package pipelines_test

import (
	"github.com/concourse/testflight/gitserver"
	"github.com/concourse/testflight/guidserver"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("A job with a task that produces outputs", func() {
	var guidServer *guidserver.Server
	var originGitServer *gitserver.Server

	BeforeEach(func() {
		guidServer = guidserver.Start(guidServerRootfs, gardenClient)
		originGitServer = gitserver.Start(gitServerRootfs, gardenClient)

		configurePipeline(
			"-c", "fixtures/task-outputs.yml",
			"-v", "git-server="+originGitServer.URI(),
		)
	})

	AfterEach(func() {
		guidServer.Stop()
		originGitServer.Stop()
	})

	It("propagates the outputs from one task to another", func() {
		originGitServer.Commit()

		watch := flyWatch("some-job")
		Expect(watch).To(gbytes.Say("initializing"))
		Expect(watch).To(gbytes.Say("./git-repo/guids"))
		Expect(watch).To(gbytes.Say("./output-1/file-1"))
		Expect(watch).To(gbytes.Say("./output-2/file-2"))
		Expect(watch).To(gexec.Exit(0))
	})
})
