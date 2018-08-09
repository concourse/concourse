package pipelines_test

import (
	"fmt"

	"github.com/concourse/testflight/gitserver"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
)

var checkContents = `#!/bin/sh
sleep 10000
`

var _ = FDescribe("A resource check which times out", func() {
	var originGitServer *gitserver.Server

	BeforeEach(func() {
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

	It("times out when checking", func() {
		watch := flyHelper.CheckResource("-r", fmt.Sprintf("%s/my-resource", pipelineName))
		<-watch.Exited
		Expect(watch).To(gbytes.Say("check timed out"))
		Expect(watch).To(gexec.Exit(1))
	})
})
