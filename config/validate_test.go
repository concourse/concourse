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

					TaskConfigPath: "some/config/path.yml",
					TaskConfig: &atc.TaskConfig{
						Image: "some-image",
					},

					Privileged: true,

					Serial: true,

					InputConfigs: []atc.JobInputConfig{
						{
							RawName:  "some-input",
							Resource: "some-resource",
							Params: atc.Params{
								"some-param": "some-value",
							},
							Passed: []string{"some-job"},
						},
					},

					OutputConfigs: []atc.JobOutputConfig{
						{
							Resource: "some-resource",
							Params: atc.Params{
								"some-param": "some-value",
							},
							RawPerformOn: []atc.Condition{"success", "failure"},
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
				Ω(validateErr.Error()).Should(ContainSubstring("resources[1] has no name"))
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
				Ω(validateErr.Error()).Should(ContainSubstring("resources.bogus-resource has no type"))
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
				Ω(validateErr.Error()).Should(ContainSubstring("resources[1] has no name"))
				Ω(validateErr.Error()).Should(ContainSubstring("resources[1] has no type"))
			})
		})

		Context("when two resources have the same name", func() {
			BeforeEach(func() {
				config.Resources = append(config.Resources, config.Resources...)
			})

			It("returns an error", func() {
				Ω(validateErr).Should(HaveOccurred())
				Ω(validateErr.Error()).Should(ContainSubstring(
					"resources[0] and resources[1] have the same name ('some-resource')",
				))
			})
		})
	})

	Describe("validating a job", func() {
		var job atc.JobConfig

		BeforeEach(func() {
			job = atc.JobConfig{
				Name:           "some-other-job",
				TaskConfigPath: "some-task-config",
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
					"jobs[1] has no name",
				))
			})
		})

		Context("when a job has no config and no config path", func() {
			BeforeEach(func() {
				job.TaskConfig = nil
				job.TaskConfigPath = ""
				config.Jobs = append(config.Jobs, job)
			})

			It("returns no error", func() {
				Ω(validateErr).ShouldNot(HaveOccurred())
			})
		})

		Context("when a job's input has no resource", func() {
			BeforeEach(func() {
				job.InputConfigs = append(job.InputConfigs, atc.JobInputConfig{
					RawName: "foo",
				})
				config.Jobs = append(config.Jobs, job)
			})

			It("returns an error", func() {
				Ω(validateErr).Should(HaveOccurred())
				Ω(validateErr.Error()).Should(ContainSubstring(
					"jobs.some-other-job.inputs.foo has no resource",
				))
			})
		})

		Context("when a job's input has a bogus resource", func() {
			BeforeEach(func() {
				job.InputConfigs = append(job.InputConfigs, atc.JobInputConfig{
					RawName:  "foo",
					Resource: "bogus-resource",
				})
				config.Jobs = append(config.Jobs, job)
			})

			It("returns an error", func() {
				Ω(validateErr).Should(HaveOccurred())
				Ω(validateErr.Error()).Should(ContainSubstring(
					"jobs.some-other-job.inputs.foo has an unknown resource ('bogus-resource')",
				))
			})
		})

		Context("when a job's input's passed constraints reference a bogus job", func() {
			BeforeEach(func() {
				job.InputConfigs = append(job.InputConfigs, atc.JobInputConfig{
					RawName:  "foo",
					Resource: "some-resource",
					Passed:   []string{"bogus-job"},
				})
				config.Jobs = append(config.Jobs, job)
			})

			It("returns an error", func() {
				Ω(validateErr).Should(HaveOccurred())
				Ω(validateErr.Error()).Should(ContainSubstring(
					"jobs.some-other-job.inputs.foo.passed references an unknown job ('bogus-job')",
				))
			})
		})

		Context("when a job's output has no resource", func() {
			BeforeEach(func() {
				job.OutputConfigs = append(job.OutputConfigs, atc.JobOutputConfig{})
				config.Jobs = append(config.Jobs, job)
			})

			It("returns an error", func() {
				Ω(validateErr).Should(HaveOccurred())
				Ω(validateErr.Error()).Should(ContainSubstring(
					"jobs.some-other-job.outputs[0] has no resource",
				))
			})
		})

		Context("when a job's output has a bogus resource", func() {
			BeforeEach(func() {
				job.OutputConfigs = append(job.OutputConfigs, atc.JobOutputConfig{
					Resource: "bogus-resource",
				})
				config.Jobs = append(config.Jobs, job)
			})

			It("returns an error", func() {
				Ω(validateErr).Should(HaveOccurred())
				Ω(validateErr.Error()).Should(ContainSubstring(
					"jobs.some-other-job.outputs[0] has an unknown resource ('bogus-resource')",
				))
			})
		})

		Describe("plans", func() {
			BeforeEach(func() {
				// clear out old-style configuration
				job.TaskConfigPath = ""
				job.TaskConfig = nil
				job.InputConfigs = nil
				job.OutputConfigs = nil
			})

			Context("when multiple actions are specified in the same plan", func() {
				Context("when it's not just Get and Put", func() {
					BeforeEach(func() {
						job.Plan = append(job.Plan, atc.PlanConfig{
							Get:       "lol",
							Put:       "lol",
							Task:      "lol",
							Do:        &atc.PlanSequence{},
							Aggregate: &atc.PlanSequence{},
						})

						config.Jobs = append(config.Jobs, job)
					})

					It("returns an error", func() {
						Ω(validateErr).Should(HaveOccurred())
						Ω(validateErr.Error()).Should(ContainSubstring(
							"jobs.some-other-job.plan[0] has multiple actions specified (aggregate, do, get, put, task)",
						))
					})
				})

				Context("when it's just Get and Put", func() {
					BeforeEach(func() {
						job.Plan = append(job.Plan, atc.PlanConfig{
							Get:       "lol",
							Put:       "lol",
							Task:      "",
							Do:        nil,
							Aggregate: nil,
						})

						config.Jobs = append(config.Jobs, job)
					})

					It("does not return an error (put commands can have a get directive)", func() {
						Ω(validateErr).ShouldNot(HaveOccurred())
					})
				})
			})

			Context("when no actions are specified in the plan", func() {
				BeforeEach(func() {
					job.Plan = append(job.Plan, atc.PlanConfig{})

					config.Jobs = append(config.Jobs, job)
				})

				It("returns an error", func() {
					Ω(validateErr).Should(HaveOccurred())
					Ω(validateErr.Error()).Should(ContainSubstring(
						"jobs.some-other-job.plan[0] has no action specified",
					))
				})
			})

			Context("when a get plan has task-only fields specified", func() {
				BeforeEach(func() {
					job.Plan = append(job.Plan, atc.PlanConfig{
						Get:            "lol",
						Privileged:     true,
						TaskConfigPath: "task.yml",
					})

					config.Jobs = append(config.Jobs, job)
				})

				It("returns an error", func() {
					Ω(validateErr).Should(HaveOccurred())
					Ω(validateErr.Error()).Should(ContainSubstring(
						"jobs.some-other-job.plan[0].get.lol has invalid fields specified (privileged, file)",
					))
				})
			})

			Context("when a task plan has invalid fields specified", func() {
				BeforeEach(func() {
					no := false
					job.Plan = append(job.Plan, atc.PlanConfig{
						Task:       "lol",
						Resource:   "some-resource",
						Passed:     []string{"hi"},
						RawTrigger: &no,
					})

					config.Jobs = append(config.Jobs, job)
				})

				It("returns an error", func() {
					Ω(validateErr).Should(HaveOccurred())
					Ω(validateErr.Error()).Should(ContainSubstring(
						"jobs.some-other-job.plan[0].task.lol has invalid fields specified (resource, passed, trigger)",
					))
				})
			})

			Context("when a task plan has params specified", func() {
				BeforeEach(func() {
					job.Plan = append(job.Plan, atc.PlanConfig{
						Task:   "lol",
						Params: atc.Params{"A": "B"},
					})

					config.Jobs = append(config.Jobs, job)
				})

				It("returns an error", func() {
					Ω(validateErr).Should(HaveOccurred())
					Ω(validateErr.Error()).Should(ContainSubstring(
						"jobs.some-other-job.plan[0].task.lol specifies params, which should be config.params",
					))
				})
			})

			Context("when a task plan has neither a config or a path set", func() {
				BeforeEach(func() {
					job.Plan = append(job.Plan, atc.PlanConfig{
						Task: "lol",
					})

					config.Jobs = append(config.Jobs, job)
				})

				It("returns an error", func() {
					Ω(validateErr).Should(HaveOccurred())
					Ω(validateErr.Error()).Should(ContainSubstring(
						"jobs.some-other-job.plan[0].task.lol does not specify any task configuration",
					))
				})
			})

			Context("when a put plan has invalid fields specified", func() {
				BeforeEach(func() {
					no := false
					job.Plan = append(job.Plan, atc.PlanConfig{
						Put:            "lol",
						Passed:         []string{"get", "only"},
						RawTrigger:     &no,
						Privileged:     true,
						TaskConfigPath: "btaskyml",
					})

					config.Jobs = append(config.Jobs, job)
				})

				It("returns an error", func() {
					Ω(validateErr).Should(HaveOccurred())
					Ω(validateErr.Error()).Should(ContainSubstring(
						"jobs.some-other-job.plan[0].put.lol has invalid fields specified (passed, trigger, privileged, file)",
					))
				})
			})

			Context("when an aggregate step has a member who has no name", func() {
				BeforeEach(func() {
					job.Plan = append(job.Plan, atc.PlanConfig{
						Aggregate: &atc.PlanSequence{
							{Aggregate: &atc.PlanSequence{}},
						},
					})

					config.Jobs = append(config.Jobs, job)
				})

				It("returns an error", func() {
					Ω(validateErr).Should(HaveOccurred())
					Ω(validateErr.Error()).Should(ContainSubstring(
						"jobs.some-other-job.plan[0].aggregate[0] has no name",
					))
				})
			})

			Context("when a job's input's passed constraints reference a bogus job", func() {
				BeforeEach(func() {
					job.Plan = append(job.Plan, atc.PlanConfig{
						Get:    "lol",
						Passed: []string{"bogus-job"},
					})

					config.Jobs = append(config.Jobs, job)
				})

				It("returns an error", func() {
					Ω(validateErr).Should(HaveOccurred())
					Ω(validateErr.Error()).Should(ContainSubstring(
						"jobs.some-other-job.plan[0].get.lol.passed references an unknown job ('bogus-job')",
					))
				})
			})

			Context("when a man, a plan, a canal, panama are specified", func() {
				BeforeEach(func() {
					job.TaskConfig = &atc.TaskConfig{
						Run: atc.TaskRunConfig{
							Path: "ls",
						},
					}

					job.Plan = append(job.Plan, atc.PlanConfig{Get: "foo"})

					config.Jobs = append(config.Jobs, job)
				})

				It("returns an error", func() {
					Ω(validateErr).Should(HaveOccurred())
					Ω(validateErr.Error()).Should(ContainSubstring(
						"jobs.some-other-job has both a plan and inputs/outputs/build config specified",
					))
				})
			})
		})

		Context("when two jobs have the same name", func() {
			BeforeEach(func() {
				config.Jobs = append(config.Jobs, config.Jobs...)
			})

			It("returns an error", func() {
				Ω(validateErr).Should(HaveOccurred())
				Ω(validateErr.Error()).Should(ContainSubstring("jobs[0] and jobs[1] have the same name ('some-job')"))
			})
		})
	})
})
