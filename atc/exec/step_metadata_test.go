package exec_test

import (
	"github.com/concourse/concourse/atc/exec"

	. "github.com/onsi/ginkgo/v2"
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
					ExternalURL:  "https://www.example.com",
					CreatedBy:    "someone",
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
					"ATC_EXTERNAL_URL=https://www.example.com",
					"BUILD_URL=https://www.example.com/teams/some-team/pipelines/some-pipeline-name/jobs/some-job-name/builds/42",
					"BUILD_URL_SHORT=https://www.example.com/builds/1",
					"BUILD_CREATED_BY=someone",
				))
			})
		})

		Context("when pipeline instance vars are set", func() {
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
					ExternalURL:  "https://www.example.com",
					CreatedBy:    "someone",
					PipelineInstanceVars: map[string]any{
						"branch": "main",
						"env":    "prod",
					},
				}
			})

			It("includes instance vars in URLs as query parameters", func() {
				env := stepMetadata.Env()

				Expect(env).To(ContainElement("BUILD_ID=1"))
				Expect(env).To(ContainElement("BUILD_NAME=42"))
				Expect(env).To(ContainElement(MatchRegexp(`BUILD_PIPELINE_INSTANCE_VARS=.*branch.*main.*`)))

				// Query parameters are sorted alphabetically
				Expect(env).To(ContainElement("BUILD_URL=https://www.example.com/teams/some-team/pipelines/some-pipeline-name/jobs/some-job-name/builds/42?vars.branch=main&vars.env=prod"))
				Expect(env).To(ContainElement("BUILD_URL_SHORT=https://www.example.com/builds/1"))
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
