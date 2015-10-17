package factory_test

import (
	"github.com/concourse/atc"
	"github.com/concourse/atc/scheduler/factory"
	"github.com/concourse/atc/scheduler/factory/fakes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Factory Put", func() {

	// Due to the fact that DependentGet steps do not exist when we normally
	// bind locations, we bind them at the point we convert to a build plan -
	// so they have to be tested here, not in the LocationPopulator test
	Describe("Put/DependentGet locations", func() {
		var (
			buildFactory factory.BuildFactory

			resources atc.ResourceConfigs
			input     atc.JobConfig
		)

		BeforeEach(func() {
			locationPopulator := factory.NewLocationPopulator()

			buildFactory = factory.NewBuildFactory(
				"some-pipeline",
				locationPopulator,
			)

			resources = atc.ResourceConfigs{
				{
					Name:   "some-resource",
					Type:   "git",
					Source: atc.Source{"uri": "git://some-resource"},
				},
			}
		})

		Context("with a put at the top-level", func() {
			BeforeEach(func() {
				input = atc.JobConfig{
					Plan: atc.PlanSequence{
						{
							Put:      "some-put",
							Resource: "some-resource",
						},
					},
				}
			})

			It("returns the correct plan", func() {
				actual := buildFactory.Create(input, resources, nil)

				expected := atc.Plan{
					OnSuccess: &atc.OnSuccessPlan{
						Step: atc.Plan{
							Location: &atc.Location{
								ParentID:      0,
								ID:            1,
								ParallelGroup: 0,
							},
							Put: &atc.PutPlan{
								Type:     "git",
								Name:     "some-put",
								Resource: "some-resource",
								Pipeline: "some-pipeline",
								Source: atc.Source{
									"uri": "git://some-resource",
								},
							},
						},
						Next: atc.Plan{
							Location: &atc.Location{
								ParentID:      1,
								ID:            2,
								ParallelGroup: 0,
							},
							DependentGet: &atc.DependentGetPlan{
								Type:     "git",
								Name:     "some-put",
								Resource: "some-resource",
								Pipeline: "some-pipeline",
								Source: atc.Source{
									"uri": "git://some-resource",
								},
							},
						},
					},
				}
				Expect(actual).To(Equal(expected))
			})
		})
	})

	Describe("Put/DependentGet build plan", func() {
		var (
			fakeLocationPopulator *fakes.FakeLocationPopulator
			buildFactory          factory.BuildFactory

			resources atc.ResourceConfigs
			input     atc.JobConfig
		)

		BeforeEach(func() {
			fakeLocationPopulator = &fakes.FakeLocationPopulator{}

			buildFactory = factory.NewBuildFactory(
				"some-pipeline",
				fakeLocationPopulator,
			)

			resources = atc.ResourceConfigs{
				{
					Name:   "some-resource",
					Type:   "git",
					Source: atc.Source{"uri": "git://some-resource"},
				},
			}
		})

		Context("when I have a put at the top-level", func() {
			BeforeEach(func() {
				input = atc.JobConfig{
					Plan: atc.PlanSequence{
						{
							Put:      "some-put",
							Resource: "some-resource",
						},
					},
				}
			})

			It("returns the correct plan", func() {
				actual := buildFactory.Create(input, resources, nil)

				expected := atc.Plan{
					OnSuccess: &atc.OnSuccessPlan{
						Step: atc.Plan{
							Location: &atc.Location{},
							Put: &atc.PutPlan{
								Type:     "git",
								Name:     "some-put",
								Resource: "some-resource",
								Pipeline: "some-pipeline",
								Source: atc.Source{
									"uri": "git://some-resource",
								},
							},
						},
						Next: atc.Plan{
							Location: &atc.Location{},
							DependentGet: &atc.DependentGetPlan{
								Type:     "git",
								Name:     "some-put",
								Resource: "some-resource",
								Pipeline: "some-pipeline",
								Source: atc.Source{
									"uri": "git://some-resource",
								},
							},
						},
					},
				}
				Expect(actual).To(Equal(expected))
			})
		})

		Context("when I have a put in a hook", func() {
			BeforeEach(func() {
				input = atc.JobConfig{
					Plan: atc.PlanSequence{
						{
							Task: "some-task",
							Success: &atc.PlanConfig{
								Put: "some-put",
							},
						},
					},
				}
			})

			It("returns the correct plan", func() {
				actual := buildFactory.Create(input, resources, nil)

				expected := atc.Plan{
					OnSuccess: &atc.OnSuccessPlan{
						Step: atc.Plan{
							Task: &atc.TaskPlan{
								Name:     "some-task",
								Pipeline: "some-pipeline",
							},
						},

						Next: atc.Plan{
							OnSuccess: &atc.OnSuccessPlan{
								Step: atc.Plan{
									Location: &atc.Location{},
									Put: &atc.PutPlan{
										Name:     "some-put",
										Resource: "some-put",
										Pipeline: "some-pipeline",
									},
								},
								Next: atc.Plan{
									Location: &atc.Location{},
									DependentGet: &atc.DependentGetPlan{
										Name:     "some-put",
										Resource: "some-put",
										Pipeline: "some-pipeline",
									},
								},
							},
						},
					},
				}
				Expect(actual).To(Equal(expected))
			})
		})

		Context("when I have a put inside an aggregate", func() {
			BeforeEach(func() {
				input = atc.JobConfig{
					Plan: atc.PlanSequence{
						{
							Aggregate: &atc.PlanSequence{
								{
									Task: "some thing",
								},
								{
									Put: "some-put",
								},
							},
						},
					},
				}
			})

			It("returns the correct plan", func() {
				actual := buildFactory.Create(input, resources, nil)

				expected := atc.Plan{
					Aggregate: &atc.AggregatePlan{
						{
							Task: &atc.TaskPlan{
								Name:     "some thing",
								Pipeline: "some-pipeline",
							},
						},
						{
							OnSuccess: &atc.OnSuccessPlan{
								Step: atc.Plan{
									Location: &atc.Location{},
									Put: &atc.PutPlan{
										Name:     "some-put",
										Resource: "some-put",
										Pipeline: "some-pipeline",
									},
								},
								Next: atc.Plan{
									Location: &atc.Location{},
									DependentGet: &atc.DependentGetPlan{
										Name:     "some-put",
										Resource: "some-put",
										Pipeline: "some-pipeline",
									},
								},
							},
						},
					},
				}
				Expect(actual).To(Equal(expected))
			})
		})

		Context("when a put plan follows a task plan", func() {
			BeforeEach(func() {
				input = atc.JobConfig{
					Plan: atc.PlanSequence{
						{
							Task: "some-task",
						},
						{
							Put:      "money",
							Resource: "power",
						},
					},
				}
			})

			It("returns the correct plan", func() {
				actual := buildFactory.Create(input, resources, nil)

				expected := atc.Plan{
					OnSuccess: &atc.OnSuccessPlan{
						Step: atc.Plan{
							Task: &atc.TaskPlan{
								Name:     "some-task",
								Pipeline: "some-pipeline",
							},
						},
						Next: atc.Plan{
							OnSuccess: &atc.OnSuccessPlan{
								Step: atc.Plan{
									Location: &atc.Location{},
									Put: &atc.PutPlan{
										Name:     "money",
										Resource: "power",
										Pipeline: "some-pipeline",
									},
								},
								Next: atc.Plan{
									Location: &atc.Location{},
									DependentGet: &atc.DependentGetPlan{
										Name:     "money",
										Resource: "power",
										Pipeline: "some-pipeline",
									},
								},
							},
						},
					},
				}

				Expect(actual).To(Equal(expected))
			})
		})

		Context("when a put plan is between two task plans", func() {
			BeforeEach(func() {
				input = atc.JobConfig{
					Plan: atc.PlanSequence{
						{
							Task: "those who resist our will",
						},
						{
							Put: "some-other-other-resource",
						},
						{
							Task: "some-other-task",
						},
					},
				}
			})

			It("returns the correct plan", func() {
				expectedPlan := atc.Plan{
					OnSuccess: &atc.OnSuccessPlan{
						Step: atc.Plan{
							Task: &atc.TaskPlan{
								Name:     "those who resist our will",
								Pipeline: "some-pipeline",
							},
						},
						Next: atc.Plan{
							OnSuccess: &atc.OnSuccessPlan{
								Step: atc.Plan{
									OnSuccess: &atc.OnSuccessPlan{
										Step: atc.Plan{
											Location: &atc.Location{},
											Put: &atc.PutPlan{
												Name:     "some-other-other-resource",
												Resource: "some-other-other-resource",
												Pipeline: "some-pipeline",
												Params:   nil,
											},
										},
										Next: atc.Plan{
											Location: &atc.Location{},
											DependentGet: &atc.DependentGetPlan{
												Name:     "some-other-other-resource",
												Resource: "some-other-other-resource",
												Pipeline: "some-pipeline",
											},
										},
									},
								},
								Next: atc.Plan{
									Task: &atc.TaskPlan{
										Name:     "some-other-task",
										Pipeline: "some-pipeline",
									},
								},
							},
						},
					},
				}

				actual := buildFactory.Create(input, resources, nil)

				Expect(actual).To(Equal(expectedPlan))
			})
		})
	})
})
