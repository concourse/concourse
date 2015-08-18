package factory_test

import (
	"github.com/concourse/atc"
	"github.com/concourse/atc/scheduler/factory"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("The Location Populator: aggregate", func() {

	var (
		locationPopulator factory.LocationPopulator
		buildFactory      factory.BuildFactory

		testSeq *atc.PlanSequence
	)

	BeforeEach(func() {
		locationPopulator = factory.NewLocationPopulator()
		buildFactory = factory.NewBuildFactory("pipeline", locationPopulator)
	})

	Context("with an aggregate plan", func() {
		Context("with one aggregate", func() {
			BeforeEach(func() {
				testSeq = &atc.PlanSequence{
					{
						Aggregate: &atc.PlanSequence{
							{
								Task: "some thing",
							},
							{
								Task: "some other thing",
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
								Task: "some thing",
								Location: &atc.Location{
									ParentID:      0,
									ID:            3,
									ParallelGroup: 2,
									SerialGroup:   0,
									Hook:          "",
								},
							},
							{
								Task: "some other thing",
								Location: &atc.Location{
									ParentID:      0,
									ID:            4,
									ParallelGroup: 2,
									SerialGroup:   0,
									Hook:          "",
								},
							},
						},
					},
				}

				locationPopulator.PopulateLocations(testSeq)
				Ω(testSeq).Should(Equal(expected))
			})
		})

		Context("with nested aggregates", func() {
			BeforeEach(func() {
				testSeq = &atc.PlanSequence{
					{
						Aggregate: &atc.PlanSequence{
							{
								Task: "some thing",
							},
							{
								Aggregate: &atc.PlanSequence{
									{
										Task: "some nested thing",
									},
									{
										Task: "some nested other thing",
									},
								},
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
								Task: "some thing",
								Location: &atc.Location{
									ParentID:      0,
									ID:            3,
									ParallelGroup: 2,
									SerialGroup:   0,
									Hook:          "",
								},
							},
							{
								Aggregate: &atc.PlanSequence{
									{
										Task: "some nested thing",

										Location: &atc.Location{
											ParentID:      2,
											ID:            6,
											ParallelGroup: 5,
											SerialGroup:   0,
											Hook:          "",
										},
									},
									{
										Task: "some nested other thing",
										Location: &atc.Location{
											ParentID:      2,
											ID:            7,
											ParallelGroup: 5,
											SerialGroup:   0,
											Hook:          "",
										},
									},
								},
							},
						},
					},
				}

				locationPopulator.PopulateLocations(testSeq)
				Ω(testSeq).Should(Equal(expected))
			})
		})

		Context("with an aggregate that has steps with hooks", func() {
			BeforeEach(func() {
				testSeq = &atc.PlanSequence{
					{
						Aggregate: &atc.PlanSequence{
							{
								Task: "some thing",
								Success: &atc.PlanConfig{
									Task: "some success hook",
								},
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
								Task: "some thing",

								Location: &atc.Location{
									ParentID:      0,
									ID:            3,
									ParallelGroup: 2,
									SerialGroup:   0,
									Hook:          "",
								},
								Success: &atc.PlanConfig{
									Task: "some success hook",
									Location: &atc.Location{
										ParentID:      3,
										ID:            4,
										ParallelGroup: 0,
										SerialGroup:   0,
										Hook:          "success",
									},
								},
							},
						},
					},
				}

				locationPopulator.PopulateLocations(testSeq)
				Ω(testSeq).Should(Equal(expected))
			})
		})

		Context("with an aggregate that has hooks on itself", func() {
			BeforeEach(func() {
				testSeq = &atc.PlanSequence{
					{
						Aggregate: &atc.PlanSequence{
							{
								Task: "some thing",
							},
						},
						Success: &atc.PlanConfig{
							Task: "some success hook",
						},
					},
				}
			})

			It("populates correct locations", func() {
				expected := &atc.PlanSequence{
					{
						Aggregate: &atc.PlanSequence{
							{
								Task: "some thing",
								Location: &atc.Location{
									ParentID:      0,
									ID:            3,
									ParallelGroup: 2,
									SerialGroup:   0,
									Hook:          "",
								},
							},
						},
						Success: &atc.PlanConfig{
							Task: "some success hook",
							Location: &atc.Location{
								ParentID:      2,
								ID:            4,
								ParallelGroup: 0,
								SerialGroup:   0,
								Hook:          "success",
							},
						},
					},
				}

				locationPopulator.PopulateLocations(testSeq)
				Ω(testSeq).Should(Equal(expected))
			})
		})
	})
})
