package config_test

import (
	"github.com/concourse/atc"
	. "github.com/concourse/atc/config"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("ValidateConfig", func() {
	var (
		config atc.Config

		validateErr error
	)

	BeforeEach(func() {
		config = atc.Config{
			Groups: atc.GroupConfigs{
				{
					Name:      "some-group",
					Jobs:      []string{"some-job"},
					Resources: []string{"some-resource"},
				},
			},

			Resources: atc.ResourceConfigs{
				{
					Name: "some-resource",
					Type: "some-type",
					Source: atc.Source{
						"source-config": "some-value",
					},
				},
			},

			Jobs: atc.JobConfigs{
				{
					Name: "some-job",

					Public: true,

					BuildConfigPath: "some/config/path.yml",
					BuildConfig: &atc.BuildConfig{
						Image: "some-image",
					},

					Privileged: true,

					Serial: true,

					Inputs: []atc.JobInputConfig{
						{
							RawName:  "some-input",
							Resource: "some-resource",
							Params: atc.Params{
								"some-param": "some-value",
							},
							Passed: []string{"some-job"},
						},
					},

					Outputs: []atc.JobOutputConfig{
						{
							Resource: "some-resource",
							Params: atc.Params{
								"some-param": "some-value",
							},
							RawPerformOn: []atc.OutputCondition{"success", "failure"},
						},
					},
				},
			},
		}
	})

	JustBeforeEach(func() {
		validateErr = ValidateConfig(config)
	})

	Context("when the config is valid", func() {
		It("returns no error", func() {
			Ω(validateErr).ShouldNot(HaveOccurred())
		})
	})

	Describe("invalid groups", func() {
		Context("when the groups reference a bogus resource", func() {
			BeforeEach(func() {
				config.Groups = append(config.Groups, atc.GroupConfig{
					Name:      "bogus",
					Resources: []string{"bogus-resource"},
				})
			})

			It("returns an error", func() {
				Ω(validateErr).Should(HaveOccurred())
				Ω(validateErr.Error()).Should(ContainSubstring("unknown resource 'bogus-resource'"))
			})
		})

		Context("when the groups reference a bogus job", func() {
			BeforeEach(func() {
				config.Groups = append(config.Groups, atc.GroupConfig{
					Name: "bogus",
					Jobs: []string{"bogus-job"},
				})
			})

			It("returns an error", func() {
				Ω(validateErr).Should(HaveOccurred())
				Ω(validateErr.Error()).Should(ContainSubstring("unknown job 'bogus-job'"))
			})
		})
	})

	Describe("invalid resources", func() {
		Context("when a resource has no name", func() {
			BeforeEach(func() {
				config.Resources = append(config.Resources, atc.ResourceConfig{
					Name: "",
				})
			})

			It("returns an error", func() {
				Ω(validateErr).Should(HaveOccurred())
				Ω(validateErr.Error()).Should(ContainSubstring("resource at index 1 has no name"))
			})
		})

		Context("when a resource has no type", func() {
			BeforeEach(func() {
				config.Resources = append(config.Resources, atc.ResourceConfig{
					Name: "bogus-resource",
					Type: "",
				})
			})

			It("returns an error", func() {
				Ω(validateErr).Should(HaveOccurred())
				Ω(validateErr.Error()).Should(ContainSubstring("resource 'bogus-resource' has no type"))
			})
		})

		Context("when a resource has no name or type", func() {
			BeforeEach(func() {
				config.Resources = append(config.Resources, atc.ResourceConfig{
					Name: "",
					Type: "",
				})
			})

			It("returns an error describing both errors", func() {
				Ω(validateErr).Should(HaveOccurred())
				Ω(validateErr.Error()).Should(ContainSubstring("resource at index 1 has no name"))
				Ω(validateErr.Error()).Should(ContainSubstring("resource at index 1 has no type"))
			})
		})

		Context("when two resources have the same name", func() {
			BeforeEach(func() {
				config.Resources = append(config.Resources, config.Resources...)
			})

			It("returns an error", func() {
				Ω(validateErr).Should(HaveOccurred())
				Ω(validateErr.Error()).Should(ContainSubstring(
					"resources at index 0 and 1 have the same name ('some-resource')",
				))
			})
		})
	})

	Describe("validating a job", func() {
		var job atc.JobConfig

		BeforeEach(func() {
			job = atc.JobConfig{
				Name:            "some-other-job",
				BuildConfigPath: "some-build-config",
			}
		})

		Context("when a job has a only a name and a build config", func() {
			BeforeEach(func() {
				config.Jobs = append(config.Jobs, job)
			})

			It("returns no error", func() {
				Ω(validateErr).ShouldNot(HaveOccurred())
			})
		})

		Context("when a job has no name", func() {
			BeforeEach(func() {
				job.Name = ""
				config.Jobs = append(config.Jobs, job)
			})

			It("returns an error", func() {
				Ω(validateErr).Should(HaveOccurred())
				Ω(validateErr.Error()).Should(ContainSubstring(
					"job at index 1 has no name",
				))
			})
		})

		Context("when a job has no config and no config path", func() {
			BeforeEach(func() {
				job.BuildConfig = nil
				job.BuildConfigPath = ""
				config.Jobs = append(config.Jobs, job)
			})

			It("returns no error", func() {
				Ω(validateErr).ShouldNot(HaveOccurred())
			})
		})

		Context("when a job's input has no resource", func() {
			BeforeEach(func() {
				job.Inputs = append(job.Inputs, atc.JobInputConfig{
					RawName: "foo",
				})
				config.Jobs = append(config.Jobs, job)
			})

			It("returns an error", func() {
				Ω(validateErr).Should(HaveOccurred())
				Ω(validateErr.Error()).Should(ContainSubstring(
					"job 'some-other-job' has an input ('foo') with no resource",
				))
			})
		})

		Context("when a job's input has a bogus resource", func() {
			BeforeEach(func() {
				job.Inputs = append(job.Inputs, atc.JobInputConfig{
					RawName:  "foo",
					Resource: "bogus-resource",
				})
				config.Jobs = append(config.Jobs, job)
			})

			It("returns an error", func() {
				Ω(validateErr).Should(HaveOccurred())
				Ω(validateErr.Error()).Should(ContainSubstring(
					"job 'some-other-job' has an input ('foo') with an unknown resource ('bogus-resource')",
				))
			})
		})

		Context("when a job's input's passed constraints reference a bogus job", func() {
			BeforeEach(func() {
				job.Inputs = append(job.Inputs, atc.JobInputConfig{
					RawName:  "foo",
					Resource: "some-resource",
					Passed:   []string{"bogus-job"},
				})
				config.Jobs = append(config.Jobs, job)
			})

			It("returns an error", func() {
				Ω(validateErr).Should(HaveOccurred())
				Ω(validateErr.Error()).Should(ContainSubstring(
					"job 'some-other-job' has an input ('foo') with an unknown job dependency ('bogus-job')",
				))
			})
		})

		Context("when a job's output has no resource", func() {
			BeforeEach(func() {
				job.Outputs = append(job.Outputs, atc.JobOutputConfig{})
				config.Jobs = append(config.Jobs, job)
			})

			It("returns an error", func() {
				Ω(validateErr).Should(HaveOccurred())
				Ω(validateErr.Error()).Should(ContainSubstring(
					"job 'some-other-job' has an output (at index 0) with no resource",
				))
			})
		})

		Context("when a job's output has a bogus resource", func() {
			BeforeEach(func() {
				job.Outputs = append(job.Outputs, atc.JobOutputConfig{
					Resource: "bogus-resource",
				})
				config.Jobs = append(config.Jobs, job)
			})

			It("returns an error", func() {
				Ω(validateErr).Should(HaveOccurred())
				Ω(validateErr.Error()).Should(ContainSubstring(
					"job 'some-other-job' has an output (at index 0) with an unknown resource ('bogus-resource')",
				))
			})
		})

		Context("when two jobs have the same name", func() {
			BeforeEach(func() {
				config.Jobs = append(config.Jobs, config.Jobs...)
			})

			It("returns an error", func() {
				Ω(validateErr).Should(HaveOccurred())
				Ω(validateErr.Error()).Should(ContainSubstring("jobs at index 0 and 1 have the same name ('some-job')"))
			})
		})
	})
})
