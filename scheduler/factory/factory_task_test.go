package factory_test

import (
	"github.com/concourse/atc"
	"github.com/concourse/atc/scheduler/factory"
	"github.com/concourse/atc/testhelpers"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Factory Task", func() {
	Describe("TaskPlan", func() {
		var (
			buildFactory factory.BuildFactory

			resources           atc.ResourceConfigs
			resourceTypes       atc.ResourceTypes
			input               atc.JobConfig
			actualPlanFactory   atc.PlanFactory
			expectedPlanFactory atc.PlanFactory
			params              atc.Params
		)

		BeforeEach(func() {
			actualPlanFactory = atc.NewPlanFactory(123)
			expectedPlanFactory = atc.NewPlanFactory(123)
			buildFactory = factory.NewBuildFactory("some-pipeline", actualPlanFactory)

			resources = atc.ResourceConfigs{
				{
					Name:   "some-resource",
					Type:   "git",
					Source: atc.Source{"uri": "git://some-resource"},
				},
			}

			resourceTypes = atc.ResourceTypes{
				{
					Name:   "some-custom-resource",
					Type:   "docker-image",
					Source: atc.Source{"some": "custom-source"},
				},
			}
		})

		Context("with a put at the top-level", func() {
			BeforeEach(func() {
				params = atc.Params{
					"foo": "bar",
					"baz": "qux",
				}

				input = atc.JobConfig{
					Plan: atc.PlanSequence{
						{
							Task:   "some-task",
							Params: params,
						},
					},
				}
			})

			It("returns the correct plan", func() {
				actual, err := buildFactory.Create(input, resources, resourceTypes, nil)
				Expect(err).NotTo(HaveOccurred())

				expected := expectedPlanFactory.NewPlan(atc.TaskPlan{
					Name:          "some-task",
					Pipeline:      "some-pipeline",
					ResourceTypes: resourceTypes,
					Params:        params,
				})
				Expect(actual).To(testhelpers.MatchPlan(expected))
			})
		})
	})
})
