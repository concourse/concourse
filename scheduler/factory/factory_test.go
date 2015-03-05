package factory_test

import (
	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
	. "github.com/concourse/atc/scheduler/factory"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Factory", func() {
	var (
		factory *BuildFactory

		job       atc.JobConfig
		resources atc.ResourceConfigs

		expectedPlan atc.Plan
	)

	BeforeEach(func() {
		factory = &BuildFactory{}

		job = atc.JobConfig{
			Name: "some-job",

			BuildConfig: &atc.BuildConfig{
				Image: "some-image",
				Params: map[string]string{
					"FOO": "1",
					"BAR": "2",
				},
				Run: atc.BuildRunConfig{
					Path: "some-script",
					Args: []string{"arg1", "arg2"},
				},
			},

			Privileged: true,

			BuildConfigPath: "some-input/build.yml",

			Inputs: []atc.JobInputConfig{
				{
					RawName:  "some-input",
					Resource: "some-resource",
					Params:   atc.Params{"some": "params"},
				},
			},

			Outputs: []atc.JobOutputConfig{
				{
					Resource:     "some-resource",
					Params:       atc.Params{"foo": "bar"},
					RawPerformOn: []atc.OutputCondition{"success"},
				},
				{
					Resource:     "some-other-resource",
					Params:       atc.Params{"foo": "bar"},
					RawPerformOn: []atc.OutputCondition{"failure"},
				},
				{
					Resource:     "some-other-other-resource",
					Params:       atc.Params{"foo": "bar"},
					RawPerformOn: []atc.OutputCondition{},
				},
			},
		}

		expectedPlan = atc.Plan{
			Compose: &atc.ComposePlan{
				A: atc.Plan{
					Aggregate: &atc.AggregatePlan{
						"some-input": atc.Plan{
							Get: &atc.GetPlan{
								Type:     "git",
								Name:     "some-input",
								Resource: "some-resource",
								Source:   atc.Source{"uri": "git://some-resource"},
								Params:   atc.Params{"some": "params"},
							},
						},
					},
				},
				B: atc.Plan{
					Compose: &atc.ComposePlan{
						A: atc.Plan{
							Execute: &atc.ExecutePlan{
								Privileged: true,

								ConfigPath: "some-input/build.yml",
								Config: &atc.BuildConfig{
									Image: "some-image",

									Params: map[string]string{
										"FOO": "1",
										"BAR": "2",
									},

									Run: atc.BuildRunConfig{
										Path: "some-script",
										Args: []string{"arg1", "arg2"},
									},
								},
							},
						},
						B: atc.Plan{
							Aggregate: &atc.AggregatePlan{
								"some-resource": atc.Plan{
									Conditional: &atc.ConditionalPlan{
										Conditions: []atc.OutputCondition{atc.OutputConditionSuccess},
										Plan: atc.Plan{
											Put: &atc.PutPlan{
												Resource: "some-resource",
												Type:     "git",
												Params:   atc.Params{"foo": "bar"},
												Source:   atc.Source{"uri": "git://some-resource"},
											},
										},
									},
								},
								"some-other-resource": atc.Plan{
									Conditional: &atc.ConditionalPlan{
										Conditions: []atc.OutputCondition{atc.OutputConditionFailure},
										Plan: atc.Plan{
											Put: &atc.PutPlan{
												Resource: "some-other-resource",
												Type:     "git",
												Params:   atc.Params{"foo": "bar"},
												Source:   atc.Source{"uri": "git://some-other-resource"},
											},
										},
									},
								},
								"some-other-other-resource": atc.Plan{
									Conditional: &atc.ConditionalPlan{
										Conditions: []atc.OutputCondition{},
										Plan: atc.Plan{
											Put: &atc.PutPlan{
												Resource: "some-other-other-resource",
												Type:     "git",
												Params:   atc.Params{"foo": "bar"},
												Source:   atc.Source{"uri": "git://some-other-other-resource"},
											},
										},
									},
								},
							},
						},
					},
				},
			},
		}

		resources = atc.ResourceConfigs{
			{
				Name:   "some-resource",
				Type:   "git",
				Source: atc.Source{"uri": "git://some-resource"},
			},
			{
				Name:   "some-other-resource",
				Type:   "git",
				Source: atc.Source{"uri": "git://some-other-resource"},
			},
			{
				Name:   "some-other-other-resource",
				Type:   "git",
				Source: atc.Source{"uri": "git://some-other-other-resource"},
			},
			{
				Name:   "some-dependant-resource",
				Type:   "git",
				Source: atc.Source{"uri": "git://some-dependant-resource"},
			},
			{
				Name:   "some-output-resource",
				Type:   "git",
				Source: atc.Source{"uri": "git://some-output-resource"},
			},
			{
				Name:   "some-resource-with-longer-name",
				Type:   "git",
				Source: atc.Source{"uri": "git://some-resource-with-longer-name"},
			},
			{
				Name:   "some-named-resource",
				Type:   "git",
				Source: atc.Source{"uri": "git://some-named-resource"},
			},
		}
	})

	It("creates a build plan based on the job's configuration", func() {
		plan, err := factory.Create(job, resources, nil)
		Ω(err).ShouldNot(HaveOccurred())

		Ω(plan).Should(Equal(expectedPlan))
	})

	Context("when no build config is present", func() {
		BeforeEach(func() {
			job.BuildConfig = nil
			job.BuildConfigPath = ""

			expectedPlan.Compose.B.Compose.B.Aggregate = &atc.AggregatePlan{
				"some-resource": atc.Plan{
					Put: &atc.PutPlan{
						Resource: "some-resource",
						Type:     "git",
						Params:   atc.Params{"foo": "bar"},
						Source:   atc.Source{"uri": "git://some-resource"},
					},
				},
				"some-other-resource": atc.Plan{
					Put: &atc.PutPlan{
						Resource: "some-other-resource",
						Type:     "git",
						Params:   atc.Params{"foo": "bar"},
						Source:   atc.Source{"uri": "git://some-other-resource"},
					},
				},
				"some-other-other-resource": atc.Plan{
					Put: &atc.PutPlan{
						Resource: "some-other-other-resource",
						Type:     "git",
						Params:   atc.Params{"foo": "bar"},
						Source:   atc.Source{"uri": "git://some-other-other-resource"},
					},
				},
			}
		})

		It("performs the outputs unconditionally", func() {
			plan, err := factory.Create(job, resources, nil)
			Ω(err).ShouldNot(HaveOccurred())

			Ω(plan.Compose.B.Compose.B.Aggregate).Should(Equal(expectedPlan.Compose.B.Compose.B.Aggregate))
		})
	})

	Context("when an input has an explicit name", func() {
		BeforeEach(func() {
			job.Inputs = append(job.Inputs, atc.JobInputConfig{
				RawName:  "some-named-input",
				Resource: "some-named-resource",
				Params:   atc.Params{"some": "named-params"},
			})

			(*expectedPlan.Compose.A.Aggregate)["some-named-input"] = atc.Plan{
				Get: &atc.GetPlan{
					Name:     "some-named-input",
					Resource: "some-named-resource",
					Type:     "git",
					Source:   atc.Source{"uri": "git://some-named-resource"},
					Params:   atc.Params{"some": "named-params"},
				},
			}
		})

		It("uses it as the name for the input", func() {
			plan, err := factory.Create(job, resources, nil)
			Ω(err).ShouldNot(HaveOccurred())

			Ω(plan.Compose.A.Aggregate).Should(Equal(expectedPlan.Compose.A.Aggregate))
		})
	})

	Context("when inputs with versions are specified", func() {
		It("uses them for the build's inputs", func() {
			plan, err := factory.Create(job, resources, []db.BuildInput{
				{
					Name: "some-input",
					VersionedResource: db.VersionedResource{
						Resource: "some-resource",
						Type:     "git-ng",
						Version:  db.Version{"version": "1"},
						Source:   db.Source{"uri": "git://some-provided-uri"},
					},
				},
			})
			Ω(err).ShouldNot(HaveOccurred())

			Ω((*plan.Compose.A.Aggregate)["some-input"].Get).Should(Equal(&atc.GetPlan{
				Name:     "some-input",
				Resource: "some-resource",
				Type:     "git-ng",
				Source:   atc.Source{"uri": "git://some-provided-uri"},
				Params:   atc.Params{"some": "params"},
				Version:  atc.Version{"version": "1"},
			}))
		})
	})

	Context("when the job's input is not found", func() {
		BeforeEach(func() {
			job.Inputs = append(job.Inputs, atc.JobInputConfig{
				Resource: "some-bogus-resource",
			})
		})

		It("returns an error", func() {
			_, err := factory.Create(job, resources, nil)
			Ω(err).Should(HaveOccurred())
		})
	})

	Context("when the job's output is not found", func() {
		BeforeEach(func() {
			job.Outputs = append(job.Outputs, atc.JobOutputConfig{
				Resource: "some-bogus-resource",
			})
		})

		It("returns an error", func() {
			_, err := factory.Create(job, resources, nil)
			Ω(err).Should(HaveOccurred())
		})
	})
})
