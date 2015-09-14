package factory_test

import (
	"github.com/concourse/atc"
	"github.com/concourse/atc/scheduler/factory"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("The Location Populator: hooks", func() {
	var (
		locationPopulator factory.LocationPopulator

		testSeq *atc.PlanSequence
	)

	BeforeEach(func() {
		locationPopulator = factory.NewLocationPopulator()
		_ = factory.NewBuildFactory("pipeline", locationPopulator)
	})

	Context("with a plan that has hooks", func() {
		Context("with a do block", func() {
			Context("that has a hook", func() {
				BeforeEach(func() {
					testSeq = &atc.PlanSequence{
						{
							Do: &atc.PlanSequence{
								{
									Task: "those who resist our will",
								},
								{
									Task: "those who also resist our will",
								},
							},
							Failure: &atc.PlanConfig{
								Task: "some other failure",
							},
						},
					}
				})

				It("populates correct locations", func() {
					expected := &atc.PlanSequence{
						{
							Do: &atc.PlanSequence{
								{
									Task: "those who resist our will",
									Location: &atc.Location{
										ParentID:      0,
										ID:            3,
										ParallelGroup: 0,
										SerialGroup:   2,
										Hook:          "",
									},
								},
								{
									Task: "those who also resist our will",
									Location: &atc.Location{
										ParentID:      0,
										ID:            4,
										ParallelGroup: 0,
										SerialGroup:   2,
										Hook:          "",
									},
								},
							},
							Failure: &atc.PlanConfig{
								Task: "some other failure",
								Location: &atc.Location{
									ParentID:      2,
									ID:            5,
									ParallelGroup: 0,
									SerialGroup:   0,
									Hook:          "failure",
								},
							},
						},
					}
					locationPopulator.PopulateLocations(testSeq)
					Ω(testSeq).Should(Equal(expected))
				})
			})

			Context("that has three steps and a hook", func() {
				BeforeEach(func() {
					testSeq = &atc.PlanSequence{

						{
							Do: &atc.PlanSequence{
								{
									Task: "those who resist our will",
								},
								{
									Task: "those who also resist our will",
								},
								{
									Task: "third task",
								},
							},
							Failure: &atc.PlanConfig{
								Task: "some other failure",
							},
						},
					}

				})

				It("populates correct locations", func() {
					expected := &atc.PlanSequence{
						{
							Do: &atc.PlanSequence{
								{
									Task: "those who resist our will",
									Location: &atc.Location{
										ParentID:      0,
										ID:            3,
										ParallelGroup: 0,
										SerialGroup:   2,
										Hook:          "",
									},
								},
								{
									Task: "those who also resist our will",
									Location: &atc.Location{
										ParentID:      0,
										ID:            4,
										ParallelGroup: 0,
										SerialGroup:   2,
										Hook:          "",
									},
								},
								{
									Task: "third task",
									Location: &atc.Location{
										ParentID:      0,
										ID:            5,
										ParallelGroup: 0,
										SerialGroup:   2,
										Hook:          "",
									},
								},
							},
							Failure: &atc.PlanConfig{
								Task: "some other failure",
								Location: &atc.Location{
									ParentID:      2,
									ID:            6,
									ParallelGroup: 0,
									SerialGroup:   0,
									Hook:          "failure",
								},
							},
						},
					}

					locationPopulator.PopulateLocations(testSeq)
					Ω(testSeq).Should(Equal(expected))
				})
			})
		})

		Context("with an aggregate in an aggregate in a hook", func() {
			BeforeEach(func() {
				testSeq = &atc.PlanSequence{
					{
						Task: "some-task",
						Success: &atc.PlanConfig{
							Aggregate: &atc.PlanSequence{
								{
									Task: "agg-task-1",
								},
								{
									Aggregate: &atc.PlanSequence{
										{
											Task: "agg-agg-task-1",
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
						Task: "some-task",
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
									Task: "agg-task-1",
									Location: &atc.Location{
										ParentID:      1,
										ID:            4,
										ParallelGroup: 3,
										SerialGroup:   0,
										Hook:          "success",
									},
								},
								{
									Aggregate: &atc.PlanSequence{
										{
											Task: "agg-agg-task-1",
											Location: &atc.Location{
												ParentID:      3,
												ID:            7,
												ParallelGroup: 6,
												SerialGroup:   0,
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

		Context("with a nested do in a hook", func() {
			BeforeEach(func() {
				testSeq = &atc.PlanSequence{
					{
						Task: "some-task",
						Success: &atc.PlanConfig{
							Do: &atc.PlanSequence{
								{
									Task: "do-task-1",
								},
							},
						},
					},
				}

			})

			It("populates correct locations", func() {
				expected := &atc.PlanSequence{
					{
						Task: "some-task",
						Location: &atc.Location{
							ParentID:      0,
							ID:            1,
							ParallelGroup: 0,
							SerialGroup:   0,
							Hook:          "",
						},
						Success: &atc.PlanConfig{
							Do: &atc.PlanSequence{
								{
									Task: "do-task-1",
									Location: &atc.Location{
										ParentID:      1,
										ID:            4,
										ParallelGroup: 0,
										SerialGroup:   3,
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

		Context("with multiple nested do blocks in hooks", func() {
			BeforeEach(func() {
				testSeq = &atc.PlanSequence{
					{
						Task: "some-task",
						Success: &atc.PlanConfig{
							Do: &atc.PlanSequence{
								{
									Task: "do-task-1",
								},
								{
									Do: &atc.PlanSequence{
										{
											Task: "do-task-2",
										},
										{
											Task: "do-task-3",
											Success: &atc.PlanConfig{
												Task: "do-task-4",
											},
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
						Task: "some-task",
						Location: &atc.Location{
							ParentID:      0,
							ID:            1,
							ParallelGroup: 0,
							SerialGroup:   0,
							Hook:          "",
						},
						Success: &atc.PlanConfig{
							Do: &atc.PlanSequence{
								{
									Task: "do-task-1",
									Location: &atc.Location{
										ParentID:      1,
										ID:            4,
										ParallelGroup: 0,
										SerialGroup:   3,
										Hook:          "success",
									},
								},
								{
									Do: &atc.PlanSequence{
										{
											Task: "do-task-2",
											Location: &atc.Location{
												ParentID:      3,
												ID:            7,
												ParallelGroup: 0,
												SerialGroup:   6,
												Hook:          "",
											},
										},
										{
											Task: "do-task-3",
											Location: &atc.Location{
												ParentID:      3,
												ID:            8,
												ParallelGroup: 0,
												SerialGroup:   6,
												Hook:          "",
											},
											Success: &atc.PlanConfig{
												Task: "do-task-4",
												Location: &atc.Location{
													ParentID:      8,
													ID:            9,
													ParallelGroup: 0,
													SerialGroup:   0,
													Hook:          "success",
												},
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

		Context("with nested aggregates in a hook", func() {
			BeforeEach(func() {
				testSeq = &atc.PlanSequence{
					{
						Task: "some-task",
						Success: &atc.PlanConfig{
							Aggregate: &atc.PlanSequence{
								{
									Task: "agg-task-1",
									Success: &atc.PlanConfig{
										Task: "agg-task-1-success",
									},
								},
								{
									Task: "agg-task-2",
								},
							},
						},
					},
				}
			})

			It("populates correct locations", func() {
				expected := &atc.PlanSequence{
					{
						Task: "some-task",
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
									Task: "agg-task-1",
									Location: &atc.Location{
										ParentID:      1,
										ID:            4,
										ParallelGroup: 3,
										SerialGroup:   0,
										Hook:          "success",
									},
									Success: &atc.PlanConfig{
										Task: "agg-task-1-success",
										Location: &atc.Location{
											ParentID:      4,
											ID:            5,
											ParallelGroup: 0,
											SerialGroup:   0,
											Hook:          "success",
										},
									},
								},
								{
									Task: "agg-task-2",
									Location: &atc.Location{
										ParentID:      1,
										ID:            6,
										ParallelGroup: 3,
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

		Context("with multiple hooks", func() {
			BeforeEach(func() {
				testSeq = &atc.PlanSequence{
					{
						Task: "those who resist our will",
						Failure: &atc.PlanConfig{
							Get: "some-resource",
							Failure: &atc.PlanConfig{
								Task: "those who still resist our will",
							},
						},
					},
				}
			})

			It("populates correct locations", func() {
				expected := &atc.PlanSequence{
					{
						Task: "those who resist our will",
						Location: &atc.Location{
							ParentID:      0,
							ID:            1,
							ParallelGroup: 0,
							SerialGroup:   0,
							Hook:          "",
						},
						Failure: &atc.PlanConfig{
							Get: "some-resource",
							Location: &atc.Location{
								ParentID:      1,
								ID:            2,
								ParallelGroup: 0,
								SerialGroup:   0,
								Hook:          "failure",
							},
							Failure: &atc.PlanConfig{
								Task: "those who still resist our will",
								Location: &atc.Location{
									ParentID:      2,
									ID:            3,
									ParallelGroup: 0,
									SerialGroup:   0,
									Hook:          "failure",
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
