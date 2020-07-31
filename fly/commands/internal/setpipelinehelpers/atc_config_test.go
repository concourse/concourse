package setpipelinehelpers_test

import (
	"fmt"
	"os"

	"github.com/concourse/concourse/atc"
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
	It("uses the right target and pipeline ref", func() {
		atcConfig := ATCConfig{
			TargetName: "my-target",
			PipelineRef: atc.PipelineRef{
				Name:         "my-pipeline",
				InstanceVars: atc.InstanceVars{"branch": "master"},
			},
		}
		expected := fmt.Sprintf("%s -t my-target unpause-pipeline -p my-pipeline/branch:master", os.Args[0])
		Expect(atcConfig.UnpausePipelineCommand()).To(Equal(expected))
	})
})
