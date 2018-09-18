package pipelines_test

import (
	"fmt"

	"time"

	"github.com/concourse/concourse/testflight/gitserver"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("A resource check which times out", func() {
	var originGitServer *gitserver.Server
	var checkContents string

	JustBeforeEach(func() {
		originGitServer = gitserver.Start(client)
		originGitServer.CommitResource()
		originGitServer.CommitFileToBranch(checkContents, "rootfs/opt/resource/check", "master")

		flyHelper.ConfigurePipeline(
			pipelineName,
			"-c", "fixtures/resource-check-timeouts.yml",
			"-v", "origin-git-server="+originGitServer.URI(),
			"-y", "privileged=true",
		)
	})

	Context("when check script times out", func() {
		BeforeEach(func() {
			checkContents = `#!/bin/sh
				sleep 50
				echo 'should not get here' > /tmp/some-random-file
			`
		})

		It("prints an error and cancels the job", func() {
			watch := flyHelper.CheckResource("-r", fmt.Sprintf("%s/my-resource", pipelineName))
			<-watch.Exited
			Expect(watch).To(gexec.Exit(1))
			Expect(watch.Err).To(gbytes.Say("Timed out after 10s while checking for new versions - perhaps increase your resource check timeout?"))

			time.Sleep(40 * time.Second)

			hijack := flyHelper.Hijack("-c", fmt.Sprintf("%s/my-resource", pipelineName), "cat", "/tmp/some-random-file")
			<-hijack.Exited
			Expect(hijack).NotTo(gbytes.Say("should not get here"))
		})
	})

	Context("when check script finishes before timeout", func() {
		BeforeEach(func() {
			checkContents = `#!/bin/sh
				echo '[{"version":"1"}]'
			`
		})

		It("succeeds if the check script returns before the timeout", func() {
			watch := flyHelper.CheckResource("-r", fmt.Sprintf("%s/my-resource", pipelineName))
			<-watch.Exited
			Expect(watch).To(gexec.Exit(0))

			hijack := flyHelper.Hijack("-c", fmt.Sprintf("%s/my-resource", pipelineName), "cat", "/tmp/some-random-file")
			<-hijack.Exited
		})
	})
})
