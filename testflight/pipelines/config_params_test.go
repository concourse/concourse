package pipelines_test

import (
	"github.com/concourse/concourse/testflight/gitserver"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

var _ = Describe("Configuring a resource in a pipeline config", func() {
	var gitServer *gitserver.Server

	BeforeEach(func() {
		gitServer = gitserver.Start(client)

		flyHelper.ConfigurePipeline(
			pipelineName,
			"-c", "fixtures/config_params.yml",
			"-v", "git-server="+gitServer.URI(),
		)

		taskFileContents := `---
platform: linux

image_resource:
  type: mock
  source: {mirror_self: true}

params:
  SOURCE_PARAM: file_source

run:
  path: sh
  args: ["-ec", "echo -n 'SOURCE_PARAM is '; printenv SOURCE_PARAM; echo ."]
`
		gitServer.WriteFile("some-repo/task.yml", taskFileContents)
		gitServer.CommitResourceWithFile("task.yml")
	})

	AfterEach(func() {
		gitServer.Stop()
	})

	Context("when specifying file in task config", func() {
		It("executes the file with params specified in file", func() {
			watch := flyHelper.TriggerJob(pipelineName, "file-test")
			<-watch.Exited
			Expect(watch.ExitCode()).To(Equal(0))
			Expect(watch).To(gbytes.Say("file_source"))
		})

		It("executes the file with job params", func() {
			watch := flyHelper.TriggerJob(pipelineName, "file-params-test")
			<-watch.Exited
			Expect(watch.ExitCode()).To(Equal(0))
			Expect(watch).To(gbytes.Say("job_params_source"))
		})
	})
})
