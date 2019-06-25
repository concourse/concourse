package exec_test

import (
	"github.com/concourse/concourse/atc/exec"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("StepMetadata", func() {
	var stepMetadata exec.StepMetadata

	Describe("Env", func() {
		Context("when populating fields", func() {
			BeforeEach(func() {
				stepMetadata = exec.StepMetadata{
					BuildID:      1,
					BuildName:    "42",
					TeamID:       2222,
					TeamName:     "some-team",
					JobID:        3333,
					JobName:      "some-job-name",
					PipelineID:   4444,
					PipelineName: "some-pipeline-name",
					ExternalURL:  "http://www.example.com",
				}
			})

			It("returns the specified values", func() {
				Expect(stepMetadata.Env()).To(ConsistOf(
					"BUILD_ID=1",
					"BUILD_NAME=42",
					"BUILD_TEAM_ID=2222",
					"BUILD_TEAM_NAME=some-team",
					"BUILD_JOB_ID=3333",
					"BUILD_JOB_NAME=some-job-name",
					"BUILD_PIPELINE_ID=4444",
					"BUILD_PIPELINE_NAME=some-pipeline-name",
					"ATC_EXTERNAL_URL=http://www.example.com",
				))
			})
		})

		Context("when fields are empty", func() {
			BeforeEach(func() {
				stepMetadata = exec.StepMetadata{
					BuildID: 1,
				}
			})
			It("does not include fields that are not set", func() {
				Expect(stepMetadata.Env()).To(Equal([]string{
					"BUILD_ID=1",
				}))
			})
		})
	})
})
