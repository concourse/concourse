package pipelines_test

import (
	"fmt"

	"github.com/concourse/testflight/gitserver"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

var _ = Describe("When a resource type depends on another resource type", func() {
	var originGitServer *gitserver.Server

	BeforeEach(func() {
		originGitServer = gitserver.Start(client)
		originGitServer.CommitResource()
		originGitServer.CommitFileToBranch("initial", "initial", "trigger")

		flyHelper.ConfigurePipeline(
			pipelineName,
			"-c", "fixtures/recursive-resource-checking.yml",
			"-v", "origin-git-server="+originGitServer.URI(),
			"-y", "privileged=false",
		)
	})

	AfterEach(func() {
		originGitServer.Stop()
	})

	It("can check for resources using a custom type", func() {
		checkResource := flyHelper.CheckResource("-r", fmt.Sprintf("%s/recursive-custom-resource", pipelineName))
		<-checkResource.Exited
		Expect(checkResource.ExitCode()).To(Equal(0))
		Expect(checkResource).To(gbytes.Say("checked 'recursive-custom-resource'"))
	})
})
