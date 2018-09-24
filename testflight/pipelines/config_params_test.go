package pipelines_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

var _ = Describe("Configuring a resource in a pipeline config", func() {
	BeforeEach(func() {
		flyHelper.ConfigurePipeline(
			pipelineName,
			"-c", "fixtures/config_params.yml",
		)
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
