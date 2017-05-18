package pipelines_test

import (
	"github.com/concourse/testflight/gitserver"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

var _ = Describe("Configuring a resource type in a pipeline config", func() {
	var originGitServer *gitserver.Server

	BeforeEach(func() {
		originGitServer = gitserver.Start(client)
		originGitServer.CommitResource()
		originGitServer.CommitFileToBranch("initial", "initial", "trigger")
	})

	AfterEach(func() {
		originGitServer.Stop()
	})

	Context("with custom resource types", func() {
		BeforeEach(func() {
			flyHelper.ConfigurePipeline(
				pipelineName,
				"-c", "fixtures/resource-types.yml",
				"-v", "origin-git-server="+originGitServer.URI(),
			)
		})

		It("can use custom resource types for 'get', 'put', and task 'image_resource's", func() {
			watch := flyHelper.Watch(pipelineName, "resource-getter")
			<-watch.Exited
			Expect(watch.ExitCode()).To(Equal(0))

			watch = flyHelper.Watch(pipelineName, "resource-putter")
			Expect(watch).To(gbytes.Say("pushing using custom resource"))
			Expect(watch).To(gbytes.Say("some-output/some-file"))

			watch = flyHelper.Watch(pipelineName, "resource-imgur")
			Expect(watch).To(gbytes.Say("fetched from custom resource"))
			Expect(watch).To(gbytes.Say("SOME_ENV=yep"))
		})
	})

	Context("when resource type named as base resource type", func() {
		BeforeEach(func() {
			flyHelper.ConfigurePipeline(
				pipelineName,
				"-c", "fixtures/resource-type-named-as-base-type.yml",
				"-v", "origin-git-server="+originGitServer.URI(),
			)
		})

		It("can use custom resource type named as base resource type", func() {
			watch := flyHelper.Watch(pipelineName, "resource-getter")
			<-watch.Exited
			Expect(watch.ExitCode()).To(Equal(0))
		})
	})
})
