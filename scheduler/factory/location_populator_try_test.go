package factory_test

import (
	"github.com/concourse/atc"
	"github.com/concourse/atc/scheduler/factory"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("The Location Populator: try", func() {

	var (
		locationPopulator factory.LocationPopulator

		testSeq *atc.PlanSequence
	)

	BeforeEach(func() {
		locationPopulator = factory.NewLocationPopulator()
		_ = factory.NewBuildFactory("pipeline", locationPopulator)
	})

	Context("with a task wrapped in a try", func() {
		BeforeEach(func() {
			testSeq = &atc.PlanSequence{
				{
					Try: &atc.PlanConfig{
						Task: "some thing",
					},
				},
			}
		})

		It("populates correct locations", func() {
			expected := &atc.PlanSequence{
				{
					Try: &atc.PlanConfig{
						Task: "some thing",
						Location: &atc.Location{
							ParentID:      0,
							ID:            2,
							ParallelGroup: 0,
							SerialGroup:   0,
							Hook:          "",
						},
					},
				},
			}

			locationPopulator.PopulateLocations(testSeq)
			Ω(testSeq).Should(Equal(expected))
		})

	})

	Context("with a try in a hook", func() {
		BeforeEach(func() {
			testSeq = &atc.PlanSequence{
				{
					Task: "first task",
					Success: &atc.PlanConfig{
						Try: &atc.PlanConfig{
							Task: "second task",
						},
					},
				},
			}
		})

		It("populates correct locations", func() {
			expected := &atc.PlanSequence{
				{
					Location: &atc.Location{
						ID:            1,
						ParentID:      0,
						ParallelGroup: 0,
					},
					Task: "first task",
					Success: &atc.PlanConfig{
						Try: &atc.PlanConfig{
							Location: &atc.Location{
								ID:            3,
								ParentID:      1,
								ParallelGroup: 0,
								Hook:          "success",
							},
							Task: "second task",
						},
					},
				},
			}

			locationPopulator.PopulateLocations(testSeq)
			Ω(testSeq).Should(Equal(expected))
		})
	})
})
