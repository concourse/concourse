package atc_test

import (
	. "github.com/concourse/atc"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("JobConfig", func() {
	yes := true
	no := false

	Describe("Inputs", func() {
		var (
			jobConfig JobConfig

			inputs []JobInput
		)

		BeforeEach(func() {
			jobConfig = JobConfig{}
		})

		JustBeforeEach(func() {
			inputs = jobConfig.Inputs()
		})

		Context("with old style inputs", func() {
			BeforeEach(func() {
				jobConfig.InputConfigs = []JobInputConfig{
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

			It("returns them as job inputs, resolving name and trigger", func() {
				Ω(inputs).Should(Equal([]JobInput{
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
					Ω(inputs).Should(Equal([]JobInput{
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
						Ω(inputs).Should(Equal([]JobInput{
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
					Ω(inputs).Should(Equal([]JobInput{
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
					Ω(inputs).Should(Equal([]JobInput{
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
					Ω(inputs).Should(Equal([]JobInput{
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

				It("returns an empty set of inputs", func() {
					Ω(inputs).Should(BeEmpty())
				})
			})
		})
	})

	Describe("Outputs", func() {
		var (
			jobConfig JobConfig

			outputs []JobOutput
		)

		BeforeEach(func() {
			jobConfig = JobConfig{}
		})

		JustBeforeEach(func() {
			outputs = jobConfig.Outputs()
		})

		Context("with old style outputs", func() {
			BeforeEach(func() {
				jobConfig.OutputConfigs = []JobOutputConfig{
					{
						Resource: "some-resource",
					},
					{
						Resource: "some-other-resource",
					},
				}
			})

			It("returns them as job inputs, with the name as the resource", func() {
				Ω(outputs).Should(Equal([]JobOutput{
					{
						Name:     "some-resource",
						Resource: "some-resource",
					},
					{
						Name:     "some-other-resource",
						Resource: "some-other-resource",
					},
				}))
			})
		})

		Context("with a build plan", func() {
			Context("with an empty plan", func() {
				BeforeEach(func() {
					jobConfig.Plan = PlanSequence{}
				})

				It("returns an empty set of outputs", func() {
					Ω(outputs).Should(BeEmpty())
				})
			})

			Context("when an overly complicated plan is configured", func() {
				BeforeEach(func() {
					jobConfig.Plan = PlanSequence{
						{
							Aggregate: &PlanSequence{
								{
									Aggregate: &PlanSequence{
										{Put: "a"},
									},
								},
								{Put: "b", Resource: "some-resource"},
								{
									Do: &PlanSequence{
										{Put: "c"},
									},
								},
							},
						},
					}
				})

				It("returns an output for all of the put plans present", func() {
					Ω(outputs).Should(Equal([]JobOutput{
						{
							Name:     "a",
							Resource: "a",
						},
						{
							Name:     "b",
							Resource: "some-resource",
						},
						{
							Name:     "c",
							Resource: "c",
						},
					}))
				})
			})

			Context("when the plan contains no puts steps", func() {
				BeforeEach(func() {
					jobConfig.Plan = PlanSequence{
						{
							Get: "some-put-plan",
						},
					}
				})

				It("returns an empty set of outputs", func() {
					Ω(outputs).Should(BeEmpty())
				})
			})
		})
	})
})
