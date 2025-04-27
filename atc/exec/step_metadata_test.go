package exec_test

import (
	"fmt"

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
					"BUILD_CREATED_BY=someone",
				))
			})
		})

		Context("when the URL would exceed GitLab's 255 character limit", func() {
			BeforeEach(func() {
				stepMetadata = exec.StepMetadata{
					BuildID:      123456789,
					BuildName:    "42",
					TeamID:       2222,
					TeamName:     "some-really-long-team-name-that-contributes-to-exceeding-the-url-length-limit",
					JobID:        3333,
					JobName:      "some-extremely-long-job-name-that-pushes-our-url-beyond-the-gitlab-limit-of-255-characters-which-causes-problems-with-the-commit-status-api",
					PipelineID:   4444,
					PipelineName: "some-very-long-pipeline-name-that-when-combined-with-other-components-will-definitely-make-our-url-longer-than-255-characters-which-is-the-gitlab-limit",
					ExternalURL:  "https://www.example.com",
					CreatedBy:    "someone",
				}
			})

			It("falls back to the short URL format using BUILD_ID", func() {
				env := stepMetadata.Env()

				// Extract BUILD_URL from the environment variables
				var buildURL string
				for _, envVar := range env {
					if len(envVar) > 10 && envVar[:10] == "BUILD_URL=" {
						buildURL = envVar[10:]
						break
					}
				}

				// Verify we're using the short format
				expectedShortURL := fmt.Sprintf("https://www.example.com/builds/%d", stepMetadata.BuildID)
				Expect(buildURL).To(Equal(expectedShortURL))
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
