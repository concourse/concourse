package factory_test

import (
	"github.com/concourse/atc"
	"github.com/concourse/atc/scheduler/factory"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("The Location Populator: do", func() {

	var (
		locationPopulator factory.LocationPopulator

		testSeq *atc.PlanSequence
	)

	BeforeEach(func() {
		locationPopulator = factory.NewLocationPopulator()
		_ = factory.NewBuildFactory("pipeline", locationPopulator)
	})

	Context("with a do plan", func() {
		BeforeEach(func() {
			testSeq = &atc.PlanSequence{
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
			}
		})

		It("populates correct locations", func() {
			expected := &atc.PlanSequence{
				{
					Do: &atc.PlanSequence{

						{
							Location: &atc.Location{
								ParentID:      0,
								ParallelGroup: 0,
								SerialGroup:   2,
								ID:            3,
								Hook:          "",
							},
							Task: "some thing",
						},
						{

							Location: &atc.Location{
								ParentID:      0,
								ParallelGroup: 0,
								SerialGroup:   2,
								ID:            4,
								Hook:          "",
							},
							Task: "some thing-2",
						},
						{
							Do: &atc.PlanSequence{
								{
									Task: "some other thing",
									Location: &atc.Location{
										ParentID:      2,
										ParallelGroup: 0,
										SerialGroup:   6,
										ID:            7,
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

		Context("that has an aggregate inside of it", func() {
			BeforeEach(func() {
				testSeq = &atc.PlanSequence{
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
				}
			})

			It("populates correct locations", func() {
				expected := &atc.PlanSequence{
					{
						Do: &atc.PlanSequence{
							{
								Task: "some thing",
								Location: &atc.Location{
									ParentID:      0,
									ParallelGroup: 0,
									SerialGroup:   2,
									ID:            3,
									Hook:          "",
								},
							},
							{
								Aggregate: &atc.PlanSequence{
									{
										Task: "some other thing",
										Location: &atc.Location{
											ParentID:      0,
											ParallelGroup: 5,
											SerialGroup:   2,
											ID:            6,
											Hook:          "",
										},
									},
								},
							},
							{
								Task: "some thing-2",
								Location: &atc.Location{
									ParentID:      0,
									ParallelGroup: 0,
									SerialGroup:   2,
									ID:            7,
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
	})

	Context("with an aggregate plan", func() {
		BeforeEach(func() {
			testSeq = &atc.PlanSequence{
				{
					Aggregate: &atc.PlanSequence{
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
							Task: "some other thing",
							Location: &atc.Location{
								ParentID:      0,
								ID:            3,
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

		Context("that has a hook", func() {
			BeforeEach(func() {
				testSeq = &atc.PlanSequence{
					{
						Aggregate: &atc.PlanSequence{
							{
								Task: "some thing",
							},
						},
						Success: &atc.PlanConfig{
							Task: "some-success-thing",
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
							Task: "some-success-thing",
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

			Context("with a do inside", func() {
				BeforeEach(func() {
					testSeq = &atc.PlanSequence{
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
					}
				})

				It("populates correct locations", func() {
					expected := &atc.PlanSequence{
						{
							Task: "starting-task",
							Location: &atc.Location{
								ParentID:      0,
								ID:            1,
								ParallelGroup: 0,
								SerialGroup:   0,
								Hook:          "",
							},
							Success: &atc.PlanConfig{
								Aggregate: &atc.PlanSequence{
									{
										Task: "some thing",
										Location: &atc.Location{
											ParentID:      1,
											ID:            4,
											ParallelGroup: 3,
											SerialGroup:   0,
											Hook:          "success",
										},
									},
									{
										Do: &atc.PlanSequence{
											{
												Task: "some other thing",
												Location: &atc.Location{
													ParentID:      1,
													ID:            7,
													ParallelGroup: 3,
													SerialGroup:   6,
													Hook:          "",
												},
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
		})
	})
})
