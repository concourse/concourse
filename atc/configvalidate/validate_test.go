package configvalidate_test

import (
	"github.com/concourse/concourse/atc/configvalidate"
	"strings"

	. "github.com/concourse/concourse/atc"

	// load dummy credential manager
	_ "github.com/concourse/concourse/atc/creds/dummy"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("ValidateConfig", func() {
	var (
		config Config

		errorMessages []string
	)

	BeforeEach(func() {
		config = Config{
			Groups: GroupConfigs{
				{
					Name:      "some-group",
					Jobs:      []string{"some-job"},
					Resources: []string{"some-resource"},
				},
				{
					Name: "some-other-group",
					Jobs: []string{"some-empty-job"},
				},
			},

			VarSources: VarSourceConfigs{},

			Resources: ResourceConfigs{
				{
					Name: "some-resource",
					Type: "some-type",
					Source: Source{
						"source-config": "some-value",
					},
				},
			},

			ResourceTypes: ResourceTypes{
				{
					Name: "some-resource-type",
					Type: "some-type",
					Source: Source{
						"source-config": "some-value",
					},
				},
			},

			Jobs: JobConfigs{
				{
					Name:   "some-job",
					Public: true,
					Serial: true,
					Plan: PlanSequence{
						{
							Get:      "some-input",
							Resource: "some-resource",
							Params: Params{
								"some-param": "some-value",
							},
						},
						{
							Task:       "some-task",
							Privileged: true,
							ConfigPath: "some/config/path.yml",
						},
						{
							Put: "some-resource",
							Params: Params{
								"some-param": "some-value",
							},
						},
						{
							SetPipeline: "some-pipeline",
							ConfigPath:  "some-file",
						},
					},
				},
				{
					Name: "some-empty-job",
				},
			},
		}
	})

	JustBeforeEach(func() {
		_, errorMessages = configvalidate.Validate(config)
	})

	Context("when the config is valid", func() {
		It("returns no error", func() {
			Expect(errorMessages).To(HaveLen(0))
		})
	})

	Describe("invalid groups", func() {
		Context("when the groups reference a bogus resource", func() {
			BeforeEach(func() {
				config.Groups = append(config.Groups, GroupConfig{
					Name:      "bogus",
					Resources: []string{"bogus-resource"},
				})
			})

			It("returns an error", func() {
				Expect(errorMessages).To(HaveLen(1))
				Expect(errorMessages[0]).To(ContainSubstring("invalid groups:"))
				Expect(errorMessages[0]).To(ContainSubstring("unknown resource 'bogus-resource'"))
			})
		})

		Context("when the groups reference a bogus job", func() {
			BeforeEach(func() {
				config.Groups = append(config.Groups, GroupConfig{
					Name: "bogus",
					Jobs: []string{"bogus-job"},
				})
			})

			It("returns an error", func() {
				Expect(errorMessages).To(HaveLen(1))
				Expect(errorMessages[0]).To(ContainSubstring("invalid groups:"))
				Expect(errorMessages[0]).To(ContainSubstring("unknown job 'bogus-job'"))
			})
		})

		Context("when there are jobs excluded from groups", func() {
			BeforeEach(func() {
				config.Jobs = append(config.Jobs, JobConfig{
					Name: "stand-alone-job",
				})
				config.Jobs = append(config.Jobs, JobConfig{
					Name: "other-stand-alone-job",
				})
			})

			It("returns an error", func() {
				Expect(errorMessages).To(HaveLen(1))
				Expect(errorMessages[0]).To(ContainSubstring("invalid groups:"))
				Expect(errorMessages[0]).To(ContainSubstring("job 'stand-alone-job' belongs to no group"))
				Expect(errorMessages[0]).To(ContainSubstring("job 'other-stand-alone-job' belongs to no group"))
			})

		})

		Context("when the groups have two duplicate names", func() {
			BeforeEach(func() {
				config.Groups = append(config.Groups, GroupConfig{
					Name: "some-group",
				})
			})

			It("returns an error", func() {
				Expect(errorMessages).To(HaveLen(1))
				Expect(errorMessages[0]).To(ContainSubstring("invalid groups:"))
				Expect(errorMessages[0]).To(ContainSubstring("'some-group' appears 2 times. Duplicate names are not allowed."))
			})
		})

		Context("when the groups have four duplicate names", func() {
			BeforeEach(func() {
				config.Groups = append(config.Groups, GroupConfig{
					Name: "some-group",
				}, GroupConfig{
					Name: "some-group",
				}, GroupConfig{
					Name: "some-group",
				})
			})

			It("returns an error", func() {
				Expect(errorMessages).To(HaveLen(1))
				errorMessage := strings.Trim(errorMessages[0], "\n")
				errorLines := strings.Split(errorMessage, "\n")
				Expect(errorLines).To(HaveLen(2))
				Expect(errorLines[0]).To(ContainSubstring("invalid groups:"))
				Expect(errorLines[1]).To(ContainSubstring("group 'some-group' appears 4 times. Duplicate names are not allowed."))
			})
		})
	})

	Describe("invalid var sources", func() {
		Context("when a var source type is invalid", func() {
			BeforeEach(func() {
				config.VarSources = append(config.VarSources, VarSourceConfig{
					Name:   "some",
					Type:   "some",
					Config: "",
				})
			})

			It("returns an error", func() {
				Expect(errorMessages).To(HaveLen(1))
				Expect(errorMessages[0]).To(ContainSubstring("unknown credential manager type: some"))
			})
		})

		Context("when config is invalid", func() {
			BeforeEach(func() {
				config.VarSources = append(config.VarSources, VarSourceConfig{
					Name:   "some",
					Type:   "dummy",
					Config: "",
				})
			})

			It("returns an error", func() {
				Expect(errorMessages).To(HaveLen(1))
				Expect(errorMessages[0]).To(ContainSubstring("failed to create credential manager some: invalid dummy credential manager config"))
			})
		})

		Context("when duplicate var source names", func() {
			BeforeEach(func() {
				config.VarSources = append(config.VarSources,
					VarSourceConfig{
						Name: "some",
						Type: "dummy",
						Config: map[string]interface{}{
							"vars": map[string]interface{}{"k2": "v2"},
						},
					},
					VarSourceConfig{
						Name: "some",
						Type: "dummy",
						Config: map[string]interface{}{
							"vars": map[string]interface{}{"k2": "v2"},
						},
					},
				)
			})

			It("returns an error", func() {
				Expect(errorMessages).To(HaveLen(1))
				Expect(errorMessages[0]).To(ContainSubstring("duplicate var_source name: some"))
			})
		})

		Context("when var source's dependency cannot be resolved", func() {
			BeforeEach(func() {
				config.VarSources = append(config.VarSources,
					VarSourceConfig{
						Name: "s1",
						Type: "dummy",
						Config: map[string]interface{}{
							"vars": map[string]interface{}{"k": "v"},
						},
					},
					VarSourceConfig{
						Name: "s2",
						Type: "dummy",
						Config: map[string]interface{}{
							"vars": map[string]interface{}{"k": "((s1:k))"},
						},
					},
					VarSourceConfig{
						Name: "s3",
						Type: "dummy",
						Config: map[string]interface{}{
							"vars": map[string]interface{}{"k": "((none:k))"},
						},
					},
				)
			})

			It("returns an error", func() {
				Expect(errorMessages).To(HaveLen(1))
				Expect(errorMessages[0]).To(ContainSubstring("could not resolve inter-dependent var sources: s3"))
			})
		})

		Context("when var source names are circular", func() {
			BeforeEach(func() {
				config.VarSources = append(config.VarSources,
					VarSourceConfig{
						Name: "s1",
						Type: "dummy",
						Config: map[string]interface{}{
							"vars": map[string]interface{}{"k": "((s3:v))"},
						},
					},
					VarSourceConfig{
						Name: "s2",
						Type: "dummy",
						Config: map[string]interface{}{
							"vars": map[string]interface{}{"k": "((s1:k))"},
						},
					},
					VarSourceConfig{
						Name: "s3",
						Type: "dummy",
						Config: map[string]interface{}{
							"vars": map[string]interface{}{"k": "((s2:k))"},
						},
					},
				)
			})

			It("returns an error", func() {
				Expect(errorMessages).To(HaveLen(1))
				Expect(errorMessages[0]).To(ContainSubstring("could not resolve inter-dependent var sources: s1, s2, s3"))
			})
		})
	})

	Describe("invalid resources", func() {
		Context("when a resource has no name", func() {
			BeforeEach(func() {
				config.Resources = append(config.Resources, ResourceConfig{
					Name: "",
				})
			})

			It("returns an error", func() {
				Expect(errorMessages).To(HaveLen(1))
				Expect(errorMessages[0]).To(ContainSubstring("invalid resources:"))
				Expect(errorMessages[0]).To(ContainSubstring("resources[1] has no name"))
			})
		})

		Context("when a resource has no type", func() {
			BeforeEach(func() {
				config.Resources = append(config.Resources, ResourceConfig{
					Name: "bogus-resource",
					Type: "",
				})
			})

			It("returns an error", func() {
				Expect(errorMessages).To(HaveLen(1))
				Expect(errorMessages[0]).To(ContainSubstring("invalid resources:"))
				Expect(errorMessages[0]).To(ContainSubstring("resources.bogus-resource has no type"))
			})
		})

		Context("when a resource has no name or type", func() {
			BeforeEach(func() {
				config.Resources = append(config.Resources, ResourceConfig{
					Name: "",
					Type: "",
				})
			})

			It("returns an error describing both errors", func() {
				Expect(errorMessages).To(HaveLen(1))
				Expect(errorMessages[0]).To(ContainSubstring("invalid resources:"))
				Expect(errorMessages[0]).To(ContainSubstring("resources[1] has no name"))
				Expect(errorMessages[0]).To(ContainSubstring("resources[1] has no type"))
			})
		})

		Context("when two resources have the same name", func() {
			BeforeEach(func() {
				config.Resources = append(config.Resources, config.Resources...)
			})

			It("returns an error", func() {
				Expect(errorMessages).To(HaveLen(1))
				Expect(errorMessages[0]).To(ContainSubstring("invalid resources:"))
				Expect(errorMessages[0]).To(ContainSubstring(
					"resources[0] and resources[1] have the same name ('some-resource')",
				))
			})
		})
	})

	Describe("unused resources", func() {
		BeforeEach(func() {
			config = Config{
				Resources: ResourceConfigs{
					{
						Name: "unused-resource",
						Type: "some-type",
					},
					{
						Name: "get",
						Type: "some-type",
					},
					{
						Name: "get-alias",
						Type: "some-type",
					},
					{
						Name: "resource",
						Type: "some-type",
					},
					{
						Name: "put",
						Type: "some-type",
					},
					{
						Name: "put-alias",
						Type: "some-type",
					},
					{
						Name: "do",
						Type: "some-type",
					},
					{
						Name: "aggregate",
						Type: "some-type",
					},
					{
						Name: "parallel",
						Type: "some-type",
					},
					{
						Name: "abort",
						Type: "some-type",
					},
					{
						Name: "error",
						Type: "some-type",
					},
					{
						Name: "failure",
						Type: "some-type",
					},
					{
						Name: "ensure",
						Type: "some-type",
					},
					{
						Name: "success",
						Type: "some-type",
					},
					{
						Name: "try",
						Type: "some-type",
					},
					{
						Name: "another-job",
						Type: "some-type",
					},
				},

				Jobs: JobConfigs{
					{
						Name: "some-job",
						Plan: PlanSequence{
							{
								Get: "get",
							},
							{
								Get:      "get-alias",
								Resource: "resource",
							},
							{
								Put: "put",
							},
							{
								Put:      "put-alias",
								Resource: "resource",
							},
							{
								Do: &PlanSequence{
									{
										Get: "do",
									},
								},
							},
							{
								Aggregate: &PlanSequence{
									{
										Get: "aggregate",
									},
								},
							},
							{
								InParallel: &InParallelConfig{
									Steps: PlanSequence{
										{
											Get: "parallel",
										},
									},
									Limit:    1,
									FailFast: true,
								},
							},
							{
								Task:       "some-task",
								ConfigPath: "some/config/path.yml",
								Abort: &PlanConfig{
									Get: "abort",
								},
							},
							{
								Task:       "some-task",
								ConfigPath: "some/config/path.yml",
								Error: &PlanConfig{
									Get: "error",
								},
							},
							{
								Task:       "some-task",
								ConfigPath: "some/config/path.yml",
								Failure: &PlanConfig{
									Get: "failure",
								},
							},
							{
								Task:       "some-task",
								ConfigPath: "some/config/path.yml",
								Ensure: &PlanConfig{
									Get: "ensure",
								},
							},
							{
								Task:       "some-task",
								ConfigPath: "some/config/path.yml",
								Success: &PlanConfig{
									Get: "success",
								},
							},
							{
								Try: &PlanConfig{
									Get: "try",
								},
							},
							{
								Task:       "some-task",
								ConfigPath: "some/config/path.yml",
							},
						},
					},
					{
						Name: "another-job",
						Plan: PlanSequence{
							{
								Get: "another-job",
							},
							{
								Task:       "some-task",
								ConfigPath: "some/config/path.yml",
							},
						},
					},
				},
			}
		})

		Context("when a resource is not used in any jobs", func() {
			It("returns an error", func() {
				Expect(errorMessages).To(HaveLen(1))
				Expect(errorMessages[0]).To(ContainSubstring("resource 'unused-resource' is not used"))
				Expect(errorMessages[0]).To(ContainSubstring("resource 'get-alias' is not used"))
				Expect(errorMessages[0]).To(ContainSubstring("resource 'put-alias' is not used"))
			})
		})
	})

	Describe("invalid resource types", func() {
		Context("when a resource type has no name", func() {
			BeforeEach(func() {
				config.ResourceTypes = append(config.ResourceTypes, ResourceType{
					Name: "",
				})
			})

			It("returns an error", func() {
				Expect(errorMessages).To(HaveLen(1))
				Expect(errorMessages[0]).To(ContainSubstring("invalid resource types:"))
				Expect(errorMessages[0]).To(ContainSubstring("resource_types[1] has no name"))
			})
		})

		Context("when a resource has no type", func() {
			BeforeEach(func() {
				config.ResourceTypes = append(config.ResourceTypes, ResourceType{
					Name: "bogus-resource-type",
					Type: "",
				})
			})

			It("returns an error", func() {
				Expect(errorMessages).To(HaveLen(1))
				Expect(errorMessages[0]).To(ContainSubstring("invalid resource types:"))
				Expect(errorMessages[0]).To(ContainSubstring("resource_types.bogus-resource-type has no type"))
			})
		})

		Context("when a resource has no name or type", func() {
			BeforeEach(func() {
				config.ResourceTypes = append(config.ResourceTypes, ResourceType{
					Name: "",
					Type: "",
				})
			})

			It("returns an error describing both errors", func() {
				Expect(errorMessages).To(HaveLen(1))
				Expect(errorMessages[0]).To(ContainSubstring("invalid resource types:"))
				Expect(errorMessages[0]).To(ContainSubstring("resource_types[1] has no name"))
				Expect(errorMessages[0]).To(ContainSubstring("resource_types[1] has no type"))
			})
		})

		Context("when two resource types have the same name", func() {
			BeforeEach(func() {
				config.ResourceTypes = append(config.ResourceTypes, config.ResourceTypes...)
			})

			It("returns an error", func() {
				Expect(errorMessages).To(HaveLen(1))
				Expect(errorMessages[0]).To(ContainSubstring("invalid resource types:"))
				Expect(errorMessages[0]).To(ContainSubstring("resource_types[0] and resource_types[1] have the same name ('some-resource-type')"))
			})
		})
	})

	Describe("validating a job", func() {
		var job JobConfig

		BeforeEach(func() {
			job = JobConfig{
				Name: "some-other-job",
			}
			config.Groups = []GroupConfig{}
		})

		Context("when a job has no name", func() {
			BeforeEach(func() {
				job.Name = ""
				config.Jobs = append(config.Jobs, job)
			})

			It("returns an error", func() {
				Expect(errorMessages).To(HaveLen(1))
				Expect(errorMessages[0]).To(ContainSubstring("invalid jobs:"))
				Expect(errorMessages[0]).To(ContainSubstring("jobs[2] has no name"))
			})
		})

		Context("when a job has a negative build_logs_to_retain", func() {
			BeforeEach(func() {
				job.BuildLogsToRetain = -1
				config.Jobs = append(config.Jobs, job)
			})

			It("returns an error", func() {
				Expect(errorMessages).To(HaveLen(1))
				Expect(errorMessages[0]).To(ContainSubstring("invalid jobs:"))
				Expect(errorMessages[0]).To(ContainSubstring("jobs.some-other-job has negative build_logs_to_retain: -1"))
			})
		})

		Context("when a job has duplicate inputs", func() {
			BeforeEach(func() {
				job.Plan = append(job.Plan, PlanConfig{
					Get: "some-resource",
				})
				job.Plan = append(job.Plan, PlanConfig{
					Get: "some-resource",
				})

				config.Jobs = append(config.Jobs, job)
			})

			It("returns a single error", func() {
				Expect(errorMessages).To(HaveLen(1))
				Expect(errorMessages[0]).To(ContainSubstring("invalid jobs:"))
				Expect(strings.Count(errorMessages[0], "has get steps with the same name: some-resource")).To(Equal(1))
			})
		})

		Context("when a job has duplicate inputs with different resources", func() {
			BeforeEach(func() {
				job.Plan = append(job.Plan, PlanConfig{
					Get:      "some-resource",
					Resource: "a",
				})
				job.Plan = append(job.Plan, PlanConfig{
					Get:      "some-resource",
					Resource: "b",
				})

				config.Jobs = append(config.Jobs, job)
			})

			It("returns a single error", func() {
				Expect(errorMessages).To(HaveLen(1))
				Expect(errorMessages[0]).To(ContainSubstring("invalid jobs:"))
				Expect(strings.Count(errorMessages[0], "has get steps with the same name: some-resource")).To(Equal(1))
			})
		})

		Context("when a job gets the same resource multiple times but with different names", func() {
			BeforeEach(func() {
				job.Plan = append(job.Plan, PlanConfig{
					Get:      "a",
					Resource: "some-resource",
				})
				job.Plan = append(job.Plan, PlanConfig{
					Get:      "b",
					Resource: "some-resource",
				})

				config.Jobs = append(config.Jobs, job)
			})

			It("returns no errors", func() {
				Expect(errorMessages).To(HaveLen(0))
			})
		})

		Context("when a job has duplicate inputs via aggregate", func() {
			BeforeEach(func() {
				job.Plan = append(job.Plan, PlanConfig{
					Get: "some-resource",
				})
				job.Plan = append(job.Plan, PlanConfig{
					Aggregate: &PlanSequence{
						{
							Get: "some-resource",
						},
					},
				})

				config.Jobs = append(config.Jobs, job)
			})

			It("returns a single error", func() {
				Expect(errorMessages).To(HaveLen(1))
				Expect(errorMessages[0]).To(ContainSubstring("invalid jobs:"))
				Expect(strings.Count(errorMessages[0], "has get steps with the same name: some-resource")).To(Equal(1))
			})
		})

		Context("when a job has duplicate inputs via parallel", func() {
			BeforeEach(func() {
				job.Plan = append(job.Plan, PlanConfig{
					Get: "some-resource",
				})
				job.Plan = append(job.Plan, PlanConfig{
					InParallel: &InParallelConfig{
						Steps: PlanSequence{
							{
								Get: "some-resource",
							},
						},
						Limit:    1,
						FailFast: true,
					},
				})

				config.Jobs = append(config.Jobs, job)
			})

			It("returns a single error", func() {
				Expect(errorMessages).To(HaveLen(1))
				Expect(errorMessages[0]).To(ContainSubstring("invalid jobs:"))
				Expect(strings.Count(errorMessages[0], "has get steps with the same name: some-resource")).To(Equal(1))
			})
		})

		Describe("plans", func() {
			Context("when multiple actions are specified in the same plan", func() {
				Context("when it's not just Get and Put", func() {
					BeforeEach(func() {
						job.Plan = append(job.Plan, PlanConfig{
							Get:        "some-resource",
							Put:        "some-resource",
							Task:       "some-resource",
							Do:         &PlanSequence{},
							Aggregate:  &PlanSequence{},
							InParallel: &InParallelConfig{},
						})

						config.Jobs = append(config.Jobs, job)
					})

					It("returns an error", func() {
						Expect(errorMessages).To(HaveLen(1))
						Expect(errorMessages[0]).To(ContainSubstring("invalid jobs:"))
						Expect(errorMessages[0]).To(ContainSubstring("jobs.some-other-job.plan[0] has multiple actions specified (aggregate, do, get, parallel, put, task)"))
					})
				})

				Context("when it's just Get and Put (this was valid at one point)", func() {
					BeforeEach(func() {
						job.Plan = append(job.Plan, PlanConfig{
							Get:        "some-resource",
							Put:        "some-resource",
							Task:       "",
							Do:         nil,
							Aggregate:  nil,
							InParallel: nil,
						})

						config.Jobs = append(config.Jobs, job)
					})

					It("returns an error", func() {
						Expect(errorMessages).To(HaveLen(1))
						Expect(errorMessages[0]).To(ContainSubstring("invalid jobs:"))
						Expect(errorMessages[0]).To(ContainSubstring("jobs.some-other-job.plan[0] has multiple actions specified (get, put)"))
					})
				})
			})

			Context("when no actions are specified in the plan", func() {
				BeforeEach(func() {
					job.Plan = append(job.Plan, PlanConfig{})

					config.Jobs = append(config.Jobs, job)
				})

				It("returns an error", func() {
					Expect(errorMessages).To(HaveLen(1))
					Expect(errorMessages[0]).To(ContainSubstring("invalid jobs:"))
					Expect(errorMessages[0]).To(ContainSubstring("jobs.some-other-job.plan[0] has no action specified"))
				})
			})

			Context("when a get plan has task-only fields specified", func() {
				BeforeEach(func() {
					job.Plan = append(job.Plan, PlanConfig{
						Get:        "lol",
						Privileged: true,
						ConfigPath: "task.yml",
					})

					config.Jobs = append(config.Jobs, job)
				})

				It("returns an error", func() {
					Expect(errorMessages).To(HaveLen(1))
					Expect(errorMessages[0]).To(ContainSubstring("invalid jobs:"))
					Expect(errorMessages[0]).To(ContainSubstring("jobs.some-other-job.plan[0].get.lol has invalid fields specified (privileged, file)"))
				})
			})

			Context("when a task plan has invalid fields specified", func() {
				BeforeEach(func() {
					job.Plan = append(job.Plan, PlanConfig{
						Task:     "lol",
						Resource: "some-resource",
						Passed:   []string{"hi"},
						Trigger:  true,
					})

					config.Jobs = append(config.Jobs, job)
				})

				It("returns an error", func() {
					Expect(errorMessages).To(HaveLen(1))
					Expect(errorMessages[0]).To(ContainSubstring("invalid jobs:"))
					Expect(errorMessages[0]).To(ContainSubstring("jobs.some-other-job.plan[0].task.lol has invalid fields specified (resource, passed, trigger)"))
				})
			})

			Context("when a task plan has neither a config or a path set", func() {
				BeforeEach(func() {
					job.Plan = append(job.Plan, PlanConfig{
						Task:              "lol",
						ImageArtifactName: "some-image",
					})

					config.Jobs = append(config.Jobs, job)
				})

				It("returns an error", func() {
					Expect(errorMessages).To(HaveLen(1))
					Expect(errorMessages[0]).To(ContainSubstring("invalid jobs:"))
					Expect(errorMessages[0]).To(ContainSubstring("jobs.some-other-job.plan[0].task.lol does not specify any task configuration"))
				})
			})

			Context("when a task plan has config path and config specified", func() {
				BeforeEach(func() {
					job.Plan = append(job.Plan, PlanConfig{
						Task:       "lol",
						ConfigPath: "task.yml",
						TaskConfig: &TaskConfig{
							Params: TaskEnv{
								"param1": "value1",
							},
						},
					})

					config.Jobs = append(config.Jobs, job)
				})

				It("returns an error", func() {
					Expect(errorMessages).To(HaveLen(1))
					Expect(errorMessages[0]).To(ContainSubstring("invalid jobs:"))
					Expect(errorMessages[0]).To(ContainSubstring("jobs.some-other-job.plan[0].task.lol specifies both `file` and `config` in a task step"))
				})
			})

			Context("when a task plan is invalid", func() {
				BeforeEach(func() {
					job.Plan = append(job.Plan, PlanConfig{
						Task: "some-resource",
						TaskConfig: &TaskConfig{
							Params: TaskEnv{
								"param1": "value1",
							},
						},
					})

					config.Jobs = append(config.Jobs, job)
				})

				It("returns an error", func() {
					Expect(errorMessages).To(HaveLen(1))
					Expect(errorMessages[0]).To(ContainSubstring("invalid jobs:"))
					Expect(errorMessages[0]).To(ContainSubstring("jobs.some-other-job.plan[0].task.some-resource missing 'platform'"))
					Expect(errorMessages[0]).To(ContainSubstring("jobs.some-other-job.plan[0].task.some-resource missing path to executable to run"))
				})
			})

			Context("when a put plan has invalid fields specified", func() {
				BeforeEach(func() {
					job.Plan = append(job.Plan, PlanConfig{
						Put:        "lol",
						Passed:     []string{"get", "only"},
						Trigger:    true,
						Privileged: true,
						ConfigPath: "btaskyml",
					})

					config.Jobs = append(config.Jobs, job)
				})

				It("returns an error", func() {
					Expect(errorMessages).To(HaveLen(1))
					Expect(errorMessages[0]).To(ContainSubstring("invalid jobs:"))
					Expect(errorMessages[0]).To(ContainSubstring("jobs.some-other-job.plan[0].put.lol has invalid fields specified (passed, trigger, privileged, file)"))
				})
			})

			Context("when a put plan has refers to a resource that does exist", func() {
				BeforeEach(func() {
					job.Plan = append(job.Plan, PlanConfig{
						Put: "some-resource",
					})

					config.Jobs = append(config.Jobs, job)
				})

				It("does not return an error", func() {
					Expect(errorMessages).To(HaveLen(0))
				})
			})

			Context("when a get plan has refers to a resource that does not exist", func() {
				BeforeEach(func() {
					job.Plan = append(job.Plan, PlanConfig{
						Get: "some-nonexistent-resource",
					})

					config.Jobs = append(config.Jobs, job)
				})

				It("returns an error", func() {
					Expect(errorMessages).To(HaveLen(1))
					Expect(errorMessages[0]).To(ContainSubstring("invalid jobs:"))
					Expect(errorMessages[0]).To(ContainSubstring("jobs.some-other-job.plan[0].get.some-nonexistent-resource refers to a resource that does not exist"))
				})
			})

			Context("when a put plan has refers to a resource that does not exist", func() {
				BeforeEach(func() {
					job.Plan = append(job.Plan, PlanConfig{
						Put: "some-nonexistent-resource",
					})

					config.Jobs = append(config.Jobs, job)
				})

				It("returns an error", func() {
					Expect(errorMessages).To(HaveLen(1))
					Expect(errorMessages[0]).To(ContainSubstring("invalid jobs:"))
					Expect(errorMessages[0]).To(ContainSubstring("jobs.some-other-job.plan[0].put.some-nonexistent-resource refers to a resource that does not exist"))
				})
			})

			Context("when a get plan has a custom name but refers to a resource that does exist", func() {
				BeforeEach(func() {
					job.Plan = append(job.Plan, PlanConfig{
						Get:      "custom-name",
						Resource: "some-resource",
					})

					config.Jobs = append(config.Jobs, job)
				})

				It("does not return an error", func() {
					Expect(errorMessages).To(HaveLen(0))
				})
			})

			Context("when a get plan has a custom name but refers to a resource that does not exist", func() {
				BeforeEach(func() {
					job.Plan = append(job.Plan, PlanConfig{
						Get:      "custom-name",
						Resource: "some-missing-resource",
					})

					config.Jobs = append(config.Jobs, job)
				})

				It("returns an error", func() {
					Expect(errorMessages).To(HaveLen(1))
					Expect(errorMessages[0]).To(ContainSubstring("invalid jobs:"))
					Expect(errorMessages[0]).To(ContainSubstring("jobs.some-other-job.plan[0].get.custom-name refers to a resource that does not exist ('some-missing-resource')"))
				})
			})

			Context("when a put plan has a custom name but refers to a resource that does exist", func() {
				BeforeEach(func() {
					job.Plan = append(job.Plan, PlanConfig{
						Put:      "custom-name",
						Resource: "some-resource",
					})

					config.Jobs = append(config.Jobs, job)
				})

				It("does not return an error", func() {
					Expect(errorMessages).To(HaveLen(0))
				})
			})

			Context("when a job ensure hook refers to a resource that does exist", func() {
				BeforeEach(func() {
					job.Ensure = &PlanConfig{
						Get: "some-resource",
					}

					config.Jobs = append(config.Jobs, job)
				})

				It("does not return an error", func() {
					Expect(errorMessages).To(HaveLen(0))
				})
			})

			Context("when a job ensure hook refers to a resource that does not exist", func() {
				BeforeEach(func() {
					job.Ensure = &PlanConfig{
						Get: "some-nonexistent-resource",
					}

					config.Jobs = append(config.Jobs, job)
				})

				It("returns an error", func() {
					Expect(errorMessages).To(HaveLen(1))
					Expect(errorMessages[0]).To(ContainSubstring("invalid jobs:"))
					Expect(errorMessages[0]).To(ContainSubstring("jobs.some-other-job.ensure.get.some-nonexistent-resource refers to a resource that does not exist"))
				})
			})

			Context("when a get plan refers to a 'put' resource that exists in another job's hook", func() {
				var (
					job1 JobConfig
					job2 JobConfig
				)
				BeforeEach(func() {
					job1 = JobConfig{
						Name: "job-one",
					}
					job2 = JobConfig{
						Name: "job-two",
					}

					job1.Plan = append(job1.Plan, PlanConfig{
						Task: "job-one",
						Success: &PlanConfig{
							Put: "some-resource",
						},
						ConfigPath: "job-one-config-path",
					})

					job2.Plan = append(job2.Plan, PlanConfig{
						Get:    "some-resource",
						Passed: []string{"job-one"},
					})
					config.Jobs = append(config.Jobs, job1, job2)
				})

				It("does not return an error", func() {
					Expect(errorMessages).To(HaveLen(0))
				})
			})

			Context("when a get plan refers to a 'get' resource that exists in another job's hook", func() {
				var (
					job1 JobConfig
					job2 JobConfig
				)
				BeforeEach(func() {
					job1 = JobConfig{
						Name: "job-one",
					}
					job2 = JobConfig{
						Name: "job-two",
					}

					job1.Plan = append(job1.Plan, PlanConfig{
						Task: "job-one",
						Success: &PlanConfig{
							Get: "some-resource",
						},
						ConfigPath: "job-one-config-path",
					})

					job2.Plan = append(job2.Plan, PlanConfig{
						Get:    "some-resource",
						Passed: []string{"job-one"},
					})
					config.Jobs = append(config.Jobs, job1, job2)
				})

				It("does not return an error", func() {
					Expect(errorMessages).To(HaveLen(0))
				})
			})

			Context("when a get plan refers to a 'put' resource that exists in another job's try-step", func() {
				var (
					job1 JobConfig
					job2 JobConfig
				)
				BeforeEach(func() {
					job1 = JobConfig{
						Name: "job-one",
					}
					job2 = JobConfig{
						Name: "job-two",
					}

					job1.Plan = append(job1.Plan, PlanConfig{
						Try: &PlanConfig{
							Put: "some-resource",
						},
						ConfigPath: "job-one-config-path",
					})

					job2.Plan = append(job2.Plan, PlanConfig{
						Get:    "some-resource",
						Passed: []string{"job-one"},
					})
					config.Jobs = append(config.Jobs, job1, job2)

				})

				It("does not return an error", func() {
					Expect(errorMessages).To(HaveLen(0))
				})
			})

			Context("when a get plan refers to a 'get' resource that exists in another job's try-step", func() {
				var (
					job1 JobConfig
					job2 JobConfig
				)
				BeforeEach(func() {
					job1 = JobConfig{
						Name: "job-one",
					}
					job2 = JobConfig{
						Name: "job-two",
					}

					job1.Plan = append(job1.Plan, PlanConfig{
						Try: &PlanConfig{
							Get: "some-resource",
						},
						ConfigPath: "job-one-config-path",
					})

					job2.Plan = append(job2.Plan, PlanConfig{
						Get:    "some-resource",
						Passed: []string{"job-one"},
					})
					config.Jobs = append(config.Jobs, job1, job2)

				})

				It("does not return an error", func() {
					Expect(errorMessages).To(HaveLen(0))
				})
			})

			Context("when a plan has an invalid step within an abort", func() {
				BeforeEach(func() {
					job.Plan = append(job.Plan, PlanConfig{
						Get: "some-resource",
						Abort: &PlanConfig{
							Put:      "custom-name",
							Resource: "some-missing-resource",
						},
					})

					config.Jobs = append(config.Jobs, job)
				})

				It("throws a validation error", func() {
					Expect(errorMessages).To(HaveLen(1))
					Expect(errorMessages[0]).To(ContainSubstring("jobs.some-other-job.plan[0].get.some-resource.abort.put.custom-name refers to a resource that does not exist ('some-missing-resource')"))
				})
			})

			Context("when a plan has an invalid step within an error", func() {
				BeforeEach(func() {
					job.Plan = append(job.Plan, PlanConfig{
						Get: "some-resource",
						Error: &PlanConfig{
							Put:      "custom-name",
							Resource: "some-missing-resource",
						},
					})

					config.Jobs = append(config.Jobs, job)
				})

				It("throws a validation error", func() {
					Expect(errorMessages).To(HaveLen(1))
					Expect(errorMessages[0]).To(ContainSubstring("jobs.some-other-job.plan[0].get.some-resource.error.put.custom-name refers to a resource that does not exist ('some-missing-resource')"))
				})
			})

			Context("when a plan has an invalid step within an ensure", func() {
				BeforeEach(func() {
					job.Plan = append(job.Plan, PlanConfig{
						Get: "some-resource",
						Ensure: &PlanConfig{
							Put:      "custom-name",
							Resource: "some-missing-resource",
						},
					})

					config.Jobs = append(config.Jobs, job)
				})

				It("throws a validation error", func() {
					Expect(errorMessages).To(HaveLen(1))
					Expect(errorMessages[0]).To(ContainSubstring("jobs.some-other-job.plan[0].get.some-resource.ensure.put.custom-name refers to a resource that does not exist ('some-missing-resource')"))
				})
			})

			Context("when a plan has an invalid step within a success", func() {
				BeforeEach(func() {
					job.Plan = append(job.Plan, PlanConfig{
						Get: "some-resource",
						Success: &PlanConfig{
							Put:      "custom-name",
							Resource: "some-missing-resource",
						},
					})

					config.Jobs = append(config.Jobs, job)
				})

				It("throws a validation error", func() {
					Expect(errorMessages).To(HaveLen(1))
					Expect(errorMessages[0]).To(ContainSubstring("jobs.some-other-job.plan[0].get.some-resource.success.put.custom-name refers to a resource that does not exist ('some-missing-resource')"))
				})
			})

			Context("when a plan has an invalid step within a failure", func() {
				BeforeEach(func() {
					job.Plan = append(job.Plan, PlanConfig{
						Get: "some-resource",
						Failure: &PlanConfig{
							Put:      "custom-name",
							Resource: "some-missing-resource",
						},
					})

					config.Jobs = append(config.Jobs, job)
				})

				It("throws a validation error", func() {
					Expect(errorMessages).To(HaveLen(1))
					Expect(errorMessages[0]).To(ContainSubstring("jobs.some-other-job.plan[0].get.some-resource.failure.put.custom-name refers to a resource that does not exist ('some-missing-resource')"))
				})
			})

			Context("when a plan has an invalid timeout in a step", func() {
				BeforeEach(func() {
					job.Plan = append(job.Plan, PlanConfig{
						Get:     "some-resource",
						Timeout: "nope",
					})

					config.Jobs = append(config.Jobs, job)
				})

				It("throws a validation error", func() {
					Expect(errorMessages).To(HaveLen(1))
					Expect(errorMessages[0]).To(ContainSubstring("jobs.some-other-job.plan[0].get.some-resource.timeout refers to a duration that could not be parsed ('nope')"))
				})
			})

			Context("when a plan has an invalid step within a try", func() {
				BeforeEach(func() {
					job.Plan = append(job.Plan, PlanConfig{
						Try: &PlanConfig{
							Put:      "custom-name",
							Resource: "some-missing-resource",
						},
					})

					config.Jobs = append(config.Jobs, job)
				})

				It("throws a validation error", func() {
					Expect(errorMessages).To(HaveLen(1))
					Expect(errorMessages[0]).To(ContainSubstring("jobs.some-other-job.plan[0].try.put.custom-name refers to a resource that does not exist ('some-missing-resource')"))
				})
			})

			Context("when a retry plan has a negative attempts number", func() {
				BeforeEach(func() {
					job.Plan = append(job.Plan, PlanConfig{
						Put:      "some-resource",
						Attempts: -1,
					})

					config.Jobs = append(config.Jobs, job)
				})

				It("does return an error", func() {
					Expect(errorMessages).To(HaveLen(1))
					Expect(errorMessages[0]).To(ContainSubstring("jobs.some-other-job.plan[0].put.some-resource.attempts has an invalid number of attempts (-1)"))
				})
			})

			Context("when a put plan has a custom name but refers to a resource that does not exist", func() {
				BeforeEach(func() {
					job.Plan = append(job.Plan, PlanConfig{
						Put:      "custom-name",
						Resource: "some-missing-resource",
					})

					config.Jobs = append(config.Jobs, job)
				})

				It("does return an error", func() {
					Expect(errorMessages).To(HaveLen(1))
					Expect(errorMessages[0]).To(ContainSubstring("jobs.some-other-job.plan[0].put.custom-name refers to a resource that does not exist ('some-missing-resource')"))
				})
			})

			Context("when a set_pipeline step has no file configured", func() {
				BeforeEach(func() {
					job.Plan = append(job.Plan, PlanConfig{
						SetPipeline: "other-pipeline",
					})

					config.Jobs = append(config.Jobs, job)
				})

				It("does return an error", func() {
					Expect(errorMessages).To(HaveLen(1))
					Expect(errorMessages[0]).To(ContainSubstring("jobs.some-other-job.plan[0].set_pipeline.other-pipeline does not specify any pipeline configuration"))
				})
			})

			Context("when a job's input's passed constraints reference a bogus job", func() {
				BeforeEach(func() {
					job.Plan = append(job.Plan, PlanConfig{
						Get:    "lol",
						Passed: []string{"bogus-job"},
					})

					config.Jobs = append(config.Jobs, job)
				})

				It("returns an error", func() {
					Expect(errorMessages).To(HaveLen(1))
					Expect(errorMessages[0]).To(ContainSubstring("jobs.some-other-job.plan[0].get.lol.passed references an unknown job ('bogus-job')"))
				})
			})

			Context("when a job's input's passed constraints references a valid job that has the resource as an output", func() {
				BeforeEach(func() {
					config.Jobs[0].Plan = append(config.Jobs[0].Plan, PlanConfig{
						Put:      "custom-name",
						Resource: "some-resource",
					})

					job.Plan = append(job.Plan, PlanConfig{
						Get:    "some-resource",
						Passed: []string{"some-job"},
					})

					config.Jobs = append(config.Jobs, job)
				})

				It("does not return an error", func() {
					Expect(errorMessages).To(HaveLen(0))
				})
			})

			Context("when a job's input's passed constraints references a valid job that has the resource as an input", func() {
				BeforeEach(func() {
					job.Plan = append(job.Plan, PlanConfig{
						Get:    "some-resource",
						Passed: []string{"some-job"},
					})

					config.Jobs = append(config.Jobs, job)
				})

				It("does not return an error", func() {
					Expect(errorMessages).To(HaveLen(0))
				})
			})

			Context("when a job's input's passed constraints references a valid job that has the resource (with a custom name) as an input", func() {
				BeforeEach(func() {
					job.Plan = append(job.Plan, PlanConfig{
						Get:      "custom-name",
						Resource: "some-resource",
						Passed:   []string{"some-job"},
					})

					config.Jobs = append(config.Jobs, job)
				})

				It("does not return an error", func() {
					Expect(errorMessages).To(HaveLen(0))
				})
			})

			Context("when a job's input's passed constraints references a valid job that does not have the resource as an input or output", func() {
				BeforeEach(func() {
					job.Plan = append(job.Plan, PlanConfig{
						Get:    "some-resource",
						Passed: []string{"some-empty-job"},
					})

					config.Jobs = append(config.Jobs, job)
				})

				It("returns an error", func() {
					Expect(errorMessages).To(HaveLen(1))
					Expect(errorMessages[0]).To(ContainSubstring("jobs.some-other-job.plan[0].get.some-resource.passed references a job ('some-empty-job') which doesn't interact with the resource ('some-resource')"))
				})
			})
		})

		Context("when two jobs have the same name", func() {
			BeforeEach(func() {
				config.Jobs = append(config.Jobs, config.Jobs...)
			})

			It("returns an error", func() {
				Expect(errorMessages).To(HaveLen(1))
				Expect(errorMessages[0]).To(ContainSubstring("jobs[0] and jobs[2] have the same name ('some-job')"))
			})
		})

		Context("when a job has build_log_retention and deprecated build_logs_to_retain", func() {
			BeforeEach(func() {
				config.Jobs[0].BuildLogRetention = &BuildLogRetention{
					Builds: 1,
					Days:   1,
				}
				config.Jobs[0].BuildLogsToRetain = 1
			})

			It("returns an error", func() {
				Expect(errorMessages).To(HaveLen(1))
				Expect(errorMessages[0]).To(ContainSubstring("jobs.some-job can't use both build_log_retention and build_logs_to_retain"))
			})
		})

		Context("when a job has negative build_logs_to_retain", func() {
			BeforeEach(func() {
				config.Jobs[0].BuildLogsToRetain = -1
			})

			It("returns an error", func() {
				Expect(errorMessages).To(HaveLen(1))
				Expect(errorMessages[0]).To(ContainSubstring("jobs.some-job has negative build_logs_to_retain: -1"))
			})
		})

		Context("when a job has negative build_log_retention values", func() {
			BeforeEach(func() {
				config.Jobs[0].BuildLogRetention = &BuildLogRetention{
					Builds: -1,
					Days:   -1,
				}
			})

			It("returns an error", func() {
				Expect(errorMessages).To(HaveLen(1))
				Expect(errorMessages[0]).To(ContainSubstring("jobs.some-job has negative build_log_retention.builds: -1"))
				Expect(errorMessages[0]).To(ContainSubstring("jobs.some-job has negative build_log_retention.days: -1"))
			})
		})
	})
})
