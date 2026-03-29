package integration_test

import (
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/go-concourse/concourse"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/yaml"
)

var _ = Describe("Pipeline user_data", func() {
	var (
		pipelineName string
		team         concourse.Team
	)

	JustBeforeEach(func() {
		pipelineName = "pipeline-with-userdata"
		team = login(atcURL, "test", "test").Team("main")
	})

	Context("when setting a pipeline with user_data as a map", func() {
		var userDataMap map[string]any

		JustBeforeEach(func() {
			userDataMap = map[string]any{
				"organization": "Platform Team",
				"contact":      "platform@example.com",
				"labels": []any{
					"production",
					"critical",
				},
				"metadata": map[string]any{
					"cost_center": "CC-1234",
					"region":      "us-west-2",
				},
			}

			config := atc.Config{
				Jobs: atc.JobConfigs{
					{
						Name: "test-job",
						PlanSequence: []atc.Step{
							{
								Config: &atc.TaskStep{
									Name: "simple-task",
									Config: &atc.TaskConfig{
										Platform: "linux",
										Run: atc.TaskRunConfig{
											Path: "echo",
											Args: []string{"hello"},
										},
									},
								},
							},
						},
					},
				},
				UserData: userDataMap,
			}

			payload, err := yaml.Marshal(config)
			Expect(err).NotTo(HaveOccurred())

			_, _, _, err = team.CreateOrUpdatePipelineConfig(atc.PipelineRef{Name: pipelineName}, "0", payload, false)
			Expect(err).NotTo(HaveOccurred())
		})

		AfterEach(func() {
			_, err := team.DeletePipeline(atc.PipelineRef{Name: pipelineName})
			Expect(err).NotTo(HaveOccurred())
		})

		It("preserves user_data when retrieving the pipeline config", func() {
			retrievedConfig, _, _, err := team.PipelineConfig(atc.PipelineRef{Name: pipelineName})
			Expect(err).NotTo(HaveOccurred())
			Expect(retrievedConfig.UserData).NotTo(BeNil())

			userData, ok := retrievedConfig.UserData.(map[string]any)
			Expect(ok).To(BeTrue())
			Expect(userData["organization"]).To(Equal("Platform Team"))
			Expect(userData["contact"]).To(Equal("platform@example.com"))

			labels, ok := userData["labels"].([]any)
			Expect(ok).To(BeTrue())
			Expect(labels).To(ConsistOf("production", "critical"))

			metadata, ok := userData["metadata"].(map[string]any)
			Expect(ok).To(BeTrue())
			Expect(metadata["cost_center"]).To(Equal("CC-1234"))
			Expect(metadata["region"]).To(Equal("us-west-2"))
		})
	})

	Context("when setting a pipeline with user_data as a string", func() {
		JustBeforeEach(func() {
			config := atc.Config{
				Jobs: atc.JobConfigs{
					{
						Name: "test-job",
						PlanSequence: []atc.Step{
							{
								Config: &atc.TaskStep{
									Name: "simple-task",
									Config: &atc.TaskConfig{
										Platform: "linux",
										Run: atc.TaskRunConfig{
											Path: "echo",
											Args: []string{"hello"},
										},
									},
								},
							},
						},
					},
				},
				UserData: "simple string metadata",
			}

			payload, err := yaml.Marshal(config)
			Expect(err).NotTo(HaveOccurred())

			_, _, _, err = team.CreateOrUpdatePipelineConfig(atc.PipelineRef{Name: pipelineName}, "0", payload, false)
			Expect(err).NotTo(HaveOccurred())
		})

		AfterEach(func() {
			_, err := team.DeletePipeline(atc.PipelineRef{Name: pipelineName})
			Expect(err).NotTo(HaveOccurred())
		})

		It("preserves string user_data when retrieving", func() {
			retrievedConfig, _, _, err := team.PipelineConfig(atc.PipelineRef{Name: pipelineName})
			Expect(err).NotTo(HaveOccurred())
			Expect(retrievedConfig.UserData).To(Equal("simple string metadata"))
		})
	})

	Context("when setting a pipeline with user_data as an array", func() {
		JustBeforeEach(func() {
			config := atc.Config{
				Jobs: atc.JobConfigs{
					{
						Name: "test-job",
						PlanSequence: []atc.Step{
							{
								Config: &atc.TaskStep{
									Name: "simple-task",
									Config: &atc.TaskConfig{
										Platform: "linux",
										Run: atc.TaskRunConfig{
											Path: "echo",
											Args: []string{"hello"},
										},
									},
								},
							},
						},
					},
				},
				UserData: []any{"tag1", "tag2", "tag3"},
			}

			payload, err := yaml.Marshal(config)
			Expect(err).NotTo(HaveOccurred())

			_, _, _, err = team.CreateOrUpdatePipelineConfig(atc.PipelineRef{Name: pipelineName}, "0", payload, false)
			Expect(err).NotTo(HaveOccurred())
		})

		AfterEach(func() {
			_, err := team.DeletePipeline(atc.PipelineRef{Name: pipelineName})
			Expect(err).NotTo(HaveOccurred())
		})

		It("preserves array user_data when retrieving", func() {
			retrievedConfig, _, _, err := team.PipelineConfig(atc.PipelineRef{Name: pipelineName})
			Expect(err).NotTo(HaveOccurred())
			Expect(retrievedConfig.UserData).NotTo(BeNil())

			userData, ok := retrievedConfig.UserData.([]any)
			Expect(ok).To(BeTrue())
			Expect(userData).To(ConsistOf("tag1", "tag2", "tag3"))
		})
	})

	Context("when setting a pipeline without user_data", func() {
		JustBeforeEach(func() {
			config := atc.Config{
				Jobs: atc.JobConfigs{
					{
						Name: "test-job",
						PlanSequence: []atc.Step{
							{
								Config: &atc.TaskStep{
									Name: "simple-task",
									Config: &atc.TaskConfig{
										Platform: "linux",
										Run: atc.TaskRunConfig{
											Path: "echo",
											Args: []string{"hello"},
										},
									},
								},
							},
						},
					},
				},
			}

			payload, err := yaml.Marshal(config)
			Expect(err).NotTo(HaveOccurred())

			_, _, _, err = team.CreateOrUpdatePipelineConfig(atc.PipelineRef{Name: pipelineName}, "0", payload, false)
			Expect(err).NotTo(HaveOccurred())
		})

		AfterEach(func() {
			_, err := team.DeletePipeline(atc.PipelineRef{Name: pipelineName})
			Expect(err).NotTo(HaveOccurred())
		})

		It("returns nil for user_data", func() {
			retrievedConfig, _, _, err := team.PipelineConfig(atc.PipelineRef{Name: pipelineName})
			Expect(err).NotTo(HaveOccurred())
			Expect(retrievedConfig.UserData).To(BeNil())
		})
	})

	Context("when updating a pipeline's user_data", func() {
		JustBeforeEach(func() {
			config := atc.Config{
				Jobs: atc.JobConfigs{
					{
						Name: "test-job",
						PlanSequence: []atc.Step{
							{
								Config: &atc.TaskStep{
									Name: "simple-task",
									Config: &atc.TaskConfig{
										Platform: "linux",
										Run: atc.TaskRunConfig{
											Path: "echo",
											Args: []string{"hello"},
										},
									},
								},
							},
						},
					},
				},
				UserData: map[string]any{"version": "1.0"},
			}

			payload, err := yaml.Marshal(config)
			Expect(err).NotTo(HaveOccurred())

			_, _, _, err = team.CreateOrUpdatePipelineConfig(atc.PipelineRef{Name: pipelineName}, "0", payload, false)
			Expect(err).NotTo(HaveOccurred())
		})

		AfterEach(func() {
			_, err := team.DeletePipeline(atc.PipelineRef{Name: pipelineName})
			Expect(err).NotTo(HaveOccurred())
		})

		It("replaces user_data with new value", func() {
			// Retrieve current version
			currentConfig, version, _, err := team.PipelineConfig(atc.PipelineRef{Name: pipelineName})
			Expect(err).NotTo(HaveOccurred())

			// Update with new user_data
			currentConfig.UserData = map[string]any{
				"version":      "2.0",
				"updated_by":   "test-user",
				"last_updated": "2026-03-11",
			}

			payload, err := yaml.Marshal(currentConfig)
			Expect(err).NotTo(HaveOccurred())

			_, _, _, err = team.CreateOrUpdatePipelineConfig(atc.PipelineRef{Name: pipelineName}, version, payload, false)
			Expect(err).NotTo(HaveOccurred())

			// Verify updated user_data
			retrievedConfig, _, _, err := team.PipelineConfig(atc.PipelineRef{Name: pipelineName})
			Expect(err).NotTo(HaveOccurred())

			userData, ok := retrievedConfig.UserData.(map[string]any)
			Expect(ok).To(BeTrue())
			Expect(userData["version"]).To(Equal("2.0"))
			Expect(userData["updated_by"]).To(Equal("test-user"))
			Expect(userData["last_updated"]).To(Equal("2026-03-11"))
		})
	})
})
