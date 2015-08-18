package factory_test

import (
	"github.com/concourse/atc"
	"github.com/concourse/atc/scheduler/factory"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("The Location Populator: put", func() {
	var (
		locationPopulator factory.LocationPopulator
		buildFactory      factory.BuildFactory

		testSeq *atc.PlanSequence
	)

	BeforeEach(func() {
		locationPopulator = factory.NewLocationPopulator()
		buildFactory = factory.NewBuildFactory("pipeline", locationPopulator)
	})

	Context("with a put at the top-level", func() {
		BeforeEach(func() {
			testSeq = &atc.PlanSequence{
				{
					Put:      "some-put",
					Resource: "some-resource",
				},
			}
		})

		It("populates correct locations", func() {
			expected := &atc.PlanSequence{
				{
					Location: &atc.Location{
						ParentID:      0,
						ID:            1,
						ParallelGroup: 0,
					},
					Put:      "some-put",
					Resource: "some-resource",
				},
			}

			locationPopulator.PopulateLocations(testSeq)
			Ω(testSeq).Should(Equal(expected))
		})
	})

	Context("with a put in a hook", func() {
		BeforeEach(func() {
			testSeq = &atc.PlanSequence{
				{
					Task: "some-task",
					Success: &atc.PlanConfig{
						Put: "some-put",
					},
				},
			}
		})

		It("populates correct locations", func() {
			expected := &atc.PlanSequence{
				{
					Location: &atc.Location{
						ParentID:      0,
						ID:            1,
						ParallelGroup: 0,
					},
					Task: "some-task",
					Success: &atc.PlanConfig{
						Location: &atc.Location{
							ParentID:      1,
							ID:            2,
							ParallelGroup: 0,
							Hook:          "success",
						},
						Put: "some-put",
					},
				},
			}

			locationPopulator.PopulateLocations(testSeq)
			Ω(testSeq).Should(Equal(expected))
		})
	})

	Context("with a put inside an aggregate", func() {
		BeforeEach(func() {
			testSeq = &atc.PlanSequence{
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
			}
		})

		It("populates correct locations", func() {
			expected := &atc.PlanSequence{
				{
					Aggregate: &atc.PlanSequence{
						{
							Location: &atc.Location{
								ParentID:      0,
								ID:            3,
								ParallelGroup: 2,
							},
							Task: "some thing",
						},
						{
							Location: &atc.Location{
								ParentID:      0,
								ID:            4,
								ParallelGroup: 2,
							},
							Put: "some-put",
						},
					},
				},
			}

			locationPopulator.PopulateLocations(testSeq)
			Ω(testSeq).Should(Equal(expected))
		})
	})

	Context("with a put after a task", func() {
		BeforeEach(func() {
			testSeq = &atc.PlanSequence{
				{
					Task: "some-task",
				},
				{
					Put: "some-put",
				},
			}
		})

		It("populates correct locations", func() {
			expected := &atc.PlanSequence{
				{
					Location: &atc.Location{
						ParentID:      0,
						ID:            1,
						ParallelGroup: 0,
					},
					Task: "some-task",
				},
				{
					Location: &atc.Location{
						ParentID:      0,
						ID:            2,
						ParallelGroup: 0,
						Hook:          "",
					},
					Put: "some-put",
				},
			}

			locationPopulator.PopulateLocations(testSeq)
			Ω(testSeq).Should(Equal(expected))
		})
	})
})
