package pipelines_test

import (
	"github.com/concourse/testflight/gitserver"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("A job with a put that runs with no artifacts", func() {
	var originGitServer *gitserver.Server

	BeforeEach(func() {
		originGitServer = gitserver.Start(client)

		configurePipeline(
			"-c", "fixtures/put-only.yml",
			"-v", "origin-git-server="+originGitServer.URI(),
		)
	})

	AfterEach(func() {
		originGitServer.Stop()
	})

	FIt("has its working directory created anyway", func() {
		By("triggering the job")
		watch := triggerJob("broken-put")

		By("waiting for it to exit")
		<-watch.Exited

		By("asserting that it got past the 'cd' and tried to push from the bogus repository")
		Expect(watch).To(gbytes.Say("bogus: No such file or directory"))
		Expect(watch).To(gexec.Exit(1))
	})
})
