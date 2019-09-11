package setpipelinehelpers_test

import (
	"fmt"
	"os"

	. "github.com/concourse/concourse/fly/commands/internal/setpipelinehelpers"

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
		expected := fmt.Sprintf("%s -t my-target unpause-pipeline -p my-pipeline", os.Args[0])
		Expect(atcConfig.UnpausePipelineCommand()).To(Equal(expected))
	})
})
