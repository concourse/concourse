package atc_test

import (
	. "github.com/concourse/atc"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("JobConfig", func() {
	yes := true
	no := false

	Describe("BuildInputs", func() {
		var (
			jobConfig JobConfig

			inputs []JobBuildInput
		)

		BeforeEach(func() {
			jobConfig = JobConfig{}
		})

		JustBeforeEach(func() {
			inputs = jobConfig.BuildInputs()
		})

		Context("with old style inputs", func() {
			BeforeEach(func() {
				jobConfig.Inputs = []JobInputConfig{
					{
						RawName:    "some-input",
						Resource:   "some-resource",
						Passed:     []string{"a", "b"},
						RawTrigger: &yes,
					},
					{
						RawName:    "some-non-triggering-input",
						Resource:   "some-resource",
						Passed:     []string{"c", "d"},
						RawTrigger: &no,
					},
					{
						RawName:  "some-implicitly-triggering-input",
						Resource: "some-resource",
					},
				}
			})

			It("returns them directly", func() {
				Ω(inputs).Should(Equal([]JobBuildInput{
					{
						Name:     "some-input",
						Resource: "some-resource",
						Passed:   []string{"a", "b"},
						Trigger:  true,
					},
					{
						Name:     "some-non-triggering-input",
						Resource: "some-resource",
						Passed:   []string{"c", "d"},
						Trigger:  false,
					},
					{
						Name:     "some-implicitly-triggering-input",
						Resource: "some-resource",
						Trigger:  true,
					},
				}))
			})
		})

		Context("with a build plan", func() {
			Context("with an empty plan", func() {
				BeforeEach(func() {
					jobConfig.Plan = PlanSequence{}
				})

				It("returns an empty set of inputs", func() {
					Ω(inputs).Should(BeEmpty())
				})
			})

			Context("with a single get as the first step of the plan", func() {
				BeforeEach(func() {
					jobConfig.Plan = PlanSequence{
						{
							Get:        "some-get-plan",
							Passed:     []string{"a", "b"},
							RawTrigger: &yes,
						},
						{
							Get: "some-non-input-get-plan",
						},
					}
				})

				It("uses it for the inputs", func() {
					Ω(inputs).Should(Equal([]JobBuildInput{
						{
							Name:     "some-get-plan",
							Resource: "some-get-plan",
							Passed:   []string{"a", "b"},
							Trigger:  true,
						},
					}))
				})

				Context("when a resource is specified", func() {
					BeforeEach(func() {
						jobConfig.Plan = PlanSequence{
							{
								Get:      "some-get-plan",
								Resource: "some-get-resource",
							},
						}
					})

					It("uses it as resource in the input config", func() {
						Ω(inputs).Should(Equal([]JobBuildInput{
							{
								Name:     "some-get-plan",
								Resource: "some-get-resource",
								Trigger:  true,
							},
						}))
					})
				})
			})

			Context("when a simple aggregate plan is the first step", func() {
				BeforeEach(func() {
					jobConfig.Plan = PlanSequence{
						{
							Aggregate: &PlanSequence{
								{Get: "a"},
								{Get: "b", Resource: "some-resource", Passed: []string{"x"}},
								{Get: "c", RawTrigger: &no},
							},
						},
					}
				})

				It("returns an input config for all of the get plans present", func() {
					Ω(inputs).Should(Equal([]JobBuildInput{
						{
							Name:     "a",
							Resource: "a",
							Trigger:  true,
						},
						{
							Name:     "b",
							Resource: "some-resource",
							Passed:   []string{"x"},
							Trigger:  true,
						},
						{
							Name:     "c",
							Resource: "c",
							Trigger:  false,
						},
					}))
				})
			})

			Context("when a get step later in the plan has passed: constraints", func() {
				BeforeEach(func() {
					jobConfig.Plan = PlanSequence{
						{Get: "a"},
						{Put: "b"},
						{
							Aggregate: &PlanSequence{
								{Get: "c", Passed: []string{"x"}},
								{Get: "d"},
							},
						},
					}
				})

				It("returns it as an input, with trigger as 'false'", func() {
					Ω(inputs).Should(Equal([]JobBuildInput{
						{
							Name:     "a",
							Resource: "a",
							Trigger:  true,
						},
						{
							Name:     "c",
							Resource: "c",
							Passed:   []string{"x"},
							Trigger:  false,
						},
					}))
				})
			})

			Context("when an overly complicated aggregate plan is the first step", func() {
				BeforeEach(func() {
					jobConfig.Plan = PlanSequence{
						{
							Aggregate: &PlanSequence{
								{
									Aggregate: &PlanSequence{
										{Get: "a"},
									},
								},
								{Get: "b", Resource: "some-resource", Passed: []string{"x"}},
								{Get: "c", RawTrigger: &yes},
							},
						},
					}
				})

				It("returns an input config for all of the get plans present", func() {
					Ω(inputs).Should(Equal([]JobBuildInput{
						{
							Name:     "a",
							Resource: "a",
							Trigger:  true,
						},
						{
							Name:     "b",
							Resource: "some-resource",
							Passed:   []string{"x"},
							Trigger:  true,
						},
						{
							Name:     "c",
							Resource: "c",
							Trigger:  true,
						},
					}))
				})
			})

			Context("when the first step is not a get or an aggregate", func() {
				BeforeEach(func() {
					jobConfig.Plan = PlanSequence{
						{
							Put: "some-put-plan",
						},
					}
				})

				It("does not use it", func() {
					Ω(inputs).Should(BeEmpty())
				})
			})
		})
	})
})
