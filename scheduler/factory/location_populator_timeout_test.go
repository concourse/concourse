package factory_test

import (
	"github.com/concourse/atc"
	"github.com/concourse/atc/scheduler/factory"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("The Location Populator: timeout", func() {

	var (
		locationPopulator factory.LocationPopulator
		buildFactory      factory.BuildFactory

		testSeq *atc.PlanSequence
	)

	BeforeEach(func() {
		locationPopulator = factory.NewLocationPopulator()
		buildFactory = factory.NewBuildFactory("pipeline", locationPopulator)
	})

	Context("with a timeout", func() {
		BeforeEach(func() {
			testSeq = &atc.PlanSequence{
				{
					Task:    "some thing",
					Timeout: "10s",
				},
			}
		})

		It("populates correct locations", func() {
			expected := &atc.PlanSequence{
				{
					Task: "some thing",
					Location: &atc.Location{
						ParentID:      0,
						ID:            1,
						ParallelGroup: 0,
						SerialGroup:   0,
						Hook:          "",
					},
					Timeout: "10s",
				},
			}

			locationPopulator.PopulateLocations(testSeq)
			Ω(testSeq).Should(Equal(expected))
		})

	})
})
