package setpipelinehelpers_test

import (
	. "github.com/concourse/concourse/v5/fly/commands/internal/setpipelinehelpers"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("ATC Config", func() {
	Describe("Apply configuration interaction", func() {
		var atcConfig ATCConfig
		BeforeEach(func() {
			atcConfig = ATCConfig{
				SkipInteraction: true,
			}
		})

		Context("when the skip interaction flag has been set to true", func() {
			It("returns true", func() {
				Expect(atcConfig.ApplyConfigInteraction()).To(BeTrue())
			})
		})
	})

})

var _ = Describe("UnpausePipelineCommand", func() {
	It("uses the right target and pipeline name", func() {
		atcConfig := ATCConfig{
			TargetName:   "my-target",
			PipelineName: "my-pipeline",
		}
		Expect(atcConfig.UnpausePipelineCommand()).To(Equal("fly -t my-target unpause-pipeline -p my-pipeline"))
	})
})
