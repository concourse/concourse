package testflight_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

var _ = Describe("Configuring a resource in a pipeline config", func() {
	BeforeEach(func() {
		setAndUnpausePipeline("fixtures/config_params.yml")
	})

	Context("when specifying file in task config", func() {
		It("executes the file with params specified in file", func() {
			watch := fly("trigger-job", "-j", inPipeline("file-test"), "-w")
			Expect(watch).To(gbytes.Say("file_source"))
		})

		It("executes the file with job params", func() {
			watch := fly("trigger-job", "-j", inPipeline("file-params-test"), "-w")
			Expect(watch).To(gbytes.Say("job_params_source"))
		})
	})
})
