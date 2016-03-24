package pipelines_test

import (
	"time"

	"github.com/concourse/testflight/gitserver"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("Updating resource types", func() {
	var originGitServer *gitserver.Server

	BeforeEach(func() {
		originGitServer = gitserver.Start(gitServerRootfs, gardenClient)
		originGitServer.CommitResource()
		originGitServer.CommitFileToBranch("initial", "initial", "trigger")

		configurePipeline(
			"-c", "fixtures/resource-types.yml",
			"-v", "testflight-helper-image="+guidServerRootfs,
			"-v", "origin-git-server="+originGitServer.URI(),
		)
	})

	AfterEach(func() {
		originGitServer.Stop()
	})

	It("uses updated resource type", func() {
		By("watching for first resource-imgur")
		watch := flyWatch("resource-imgur", "1")
		Expect(watch).To(gbytes.Say("fetched from custom resource"))
		Expect(watch).To(gexec.Exit(0))

		originGitServer.CommitFileToBranch("new-contents", "rootfs/some-file", "master")
		originGitServer.CommitFileToBranch("new-version", "rootfs/version", "master")

		time.Sleep(10 * time.Second) // twice the default_check_interval

		By("watching for resource-imgur with updated resource type")
		originGitServer.CommitFileToBranch("trigger", "trigger", "trigger")

		watch = flyWatch("resource-imgur", "2")
		Expect(watch).To(gbytes.Say("new-contents"))
		Expect(watch).To(gexec.Exit(0))
	})
})
