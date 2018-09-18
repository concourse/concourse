package pipelines_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

var _ = Describe("A pipeline using optional inputs", func() {
	BeforeEach(func() {
		flyHelper.ConfigurePipeline(
			pipelineName,
			"-c", "fixtures/optional-inputs.yml",
		)
	})

	It("works ok even if optional inputs are missing", func() {
		watch := flyHelper.TriggerJob(pipelineName, "job-using-optional-inputs")
		<-watch.Exited

		Expect(watch).To(gbytes.Say("step 1 complete"))
		Expect(watch).To(gbytes.Say("step 2 complete"))
		Expect(watch).To(gbytes.Say("step 3 complete"))
		Expect(watch).To(gbytes.Say("SUCCESS"))
	})
})
