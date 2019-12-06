package factory_test

import (
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db/dbfakes"
	"github.com/concourse/concourse/atc/scheduler/factory"
	"github.com/concourse/concourse/atc/testhelpers"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Factory Do", func() {
	var (
		buildFactory factory.BuildFactory

		resources           atc.ResourceConfigs
		resourceTypes       atc.VersionedResourceTypes
		actualPlanFactory   atc.PlanFactory
		expectedPlanFactory atc.PlanFactory

		fakeJob *dbfakes.FakeJob
	)

	BeforeEach(func() {
		actualPlanFactory = atc.NewPlanFactory(123)
		expectedPlanFactory = atc.NewPlanFactory(123)

		buildFactory = factory.NewBuildFactory(actualPlanFactory)

		resources = atc.ResourceConfigs{
			{
				Name:   "some-resource",
				Type:   "git",
				Source: atc.Source{"uri": "git://some-resource"},
			},
		}

		resourceTypes = atc.VersionedResourceTypes{
			{
				ResourceType: atc.ResourceType{
					Name:   "some-custom-resource",
					Type:   "registry-image",
					Source: atc.Source{"some": "custom-source"},
				},
				Version: atc.Version{"some": "version"},
			},
		}

		fakeJob = new(dbfakes.FakeJob)
	})

	Context("when I have a nested do ", func() {
		BeforeEach(func() {
			fakeJob.ConfigReturns(atc.JobConfig{
				Plan: atc.PlanSequence{
					{
						Do: &atc.PlanSequence{
							{
								Task: "some thing",
							},
							{
								Task: "some thing-2",
							},
							{
								Do: &atc.PlanSequence{
									{
										Task: "some other thing",
									},
								},
							},
						},
					},
				},
			})
		})
		It("returns the correct plan", func() {
			actual, err := buildFactory.Create(fakeJob, resources, resourceTypes, nil)
			Expect(err).NotTo(HaveOccurred())

			expected := expectedPlanFactory.NewPlan(atc.DoPlan{
				expectedPlanFactory.NewPlan(atc.TaskPlan{
					Name:                   "some thing",
					VersionedResourceTypes: resourceTypes,
				}),
				expectedPlanFactory.NewPlan(atc.TaskPlan{
					Name:                   "some thing-2",
					VersionedResourceTypes: resourceTypes,
				}),
				expectedPlanFactory.NewPlan(atc.DoPlan{
					expectedPlanFactory.NewPlan(atc.TaskPlan{
						Name:                   "some other thing",
						VersionedResourceTypes: resourceTypes,
					}),
				}),
			})
			Expect(actual).To(testhelpers.MatchPlan(expected))
		})
	})

	Context("when I have an aggregate inside a do", func() {
		BeforeEach(func() {
			fakeJob.ConfigReturns(atc.JobConfig{
				Plan: atc.PlanSequence{
					{
						Do: &atc.PlanSequence{
							{
								Task: "some thing",
							},
							{
								Aggregate: &atc.PlanSequence{
									{
										Task: "some other thing",
									},
								},
							},
							{
								Task: "some thing-2",
							},
						},
					},
				},
			})
		})
		It("returns the correct plan", func() {
			actual, err := buildFactory.Create(fakeJob, resources, resourceTypes, nil)
			Expect(err).NotTo(HaveOccurred())

			expected := expectedPlanFactory.NewPlan(atc.DoPlan{
				expectedPlanFactory.NewPlan(atc.TaskPlan{
					Name:                   "some thing",
					VersionedResourceTypes: resourceTypes,
				}),
				expectedPlanFactory.NewPlan(atc.AggregatePlan{
					expectedPlanFactory.NewPlan(atc.TaskPlan{
						Name:                   "some other thing",
						VersionedResourceTypes: resourceTypes,
					}),
				}),
				expectedPlanFactory.NewPlan(atc.TaskPlan{
					Name:                   "some thing-2",
					VersionedResourceTypes: resourceTypes,
				}),
			})
			Expect(actual).To(testhelpers.MatchPlan(expected))
		})
	})

	Context("when i have a do inside an aggregate inside a hook", func() {
		BeforeEach(func() {
			fakeJob.ConfigReturns(atc.JobConfig{
				Plan: atc.PlanSequence{
					{
						Task: "starting-task",
						Success: &atc.PlanConfig{
							Aggregate: &atc.PlanSequence{
								{
									Task: "some thing",
								},
								{
									Do: &atc.PlanSequence{
										{
											Task: "some other thing",
										},
									},
								},
							},
						},
					},
				},
			})
		})
		It("returns the correct plan", func() {
			actual, err := buildFactory.Create(fakeJob, resources, resourceTypes, nil)
			Expect(err).NotTo(HaveOccurred())

			expected := expectedPlanFactory.NewPlan(atc.OnSuccessPlan{
				Step: expectedPlanFactory.NewPlan(atc.TaskPlan{
					Name:                   "starting-task",
					VersionedResourceTypes: resourceTypes,
				}),
				Next: expectedPlanFactory.NewPlan(atc.AggregatePlan{
					expectedPlanFactory.NewPlan(atc.TaskPlan{
						Name:                   "some thing",
						VersionedResourceTypes: resourceTypes,
					}),
					expectedPlanFactory.NewPlan(atc.DoPlan{
						expectedPlanFactory.NewPlan(atc.TaskPlan{
							Name:                   "some other thing",
							VersionedResourceTypes: resourceTypes,
						}),
					}),
				}),
			})

			Expect(actual).To(testhelpers.MatchPlan(expected))
		})
	})

	Context("when I have a do inside an aggregate", func() {
		BeforeEach(func() {
			fakeJob.ConfigReturns(atc.JobConfig{
				Plan: atc.PlanSequence{
					{
						Aggregate: &atc.PlanSequence{
							{
								Task: "some thing",
							},
							{
								Do: &atc.PlanSequence{
									{
										Task: "some other thing",
									},
									{
										Task: "some other thing-2",
									},
								},
							},
							{
								Task: "some thing-2",
							},
						},
					},
				},
			})
		})
		It("returns the correct plan", func() {
			actual, err := buildFactory.Create(fakeJob, resources, resourceTypes, nil)
			Expect(err).NotTo(HaveOccurred())

			expected := expectedPlanFactory.NewPlan(atc.AggregatePlan{
				expectedPlanFactory.NewPlan(atc.TaskPlan{
					Name:                   "some thing",
					VersionedResourceTypes: resourceTypes,
				}),
				expectedPlanFactory.NewPlan(atc.DoPlan{
					expectedPlanFactory.NewPlan(atc.TaskPlan{
						Name:                   "some other thing",
						VersionedResourceTypes: resourceTypes,
					}),
					expectedPlanFactory.NewPlan(atc.TaskPlan{
						Name:                   "some other thing-2",
						VersionedResourceTypes: resourceTypes,
					}),
				}),
				expectedPlanFactory.NewPlan(atc.TaskPlan{
					Name:                   "some thing-2",
					VersionedResourceTypes: resourceTypes,
				}),
			})

			Expect(actual).To(testhelpers.MatchPlan(expected))
		})
	})
})
