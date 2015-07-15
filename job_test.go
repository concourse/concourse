package atc_test

import (
	. "github.com/concourse/atc"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("JobConfig", func() {
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
						RawName:  "some-input",
						Resource: "some-resource",
						Passed:   []string{"a", "b"},
						Trigger:  true,
					},
					{
						RawName:  "some-non-triggering-input",
						Resource: "some-resource",
						Passed:   []string{"c", "d"},
						Trigger:  false,
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
						Trigger:  false,
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

			Context("with two serial gets", func() {
				BeforeEach(func() {
					jobConfig.Plan = PlanSequence{
						{
							Get:     "some-get-plan",
							Passed:  []string{"a", "b"},
							Trigger: true,
						},
						{
							Get: "some-other-get-plan",
						},
					}
				})

				It("uses both for inputs", func() {
					Ω(inputs).Should(Equal([]JobInput{
						{
							Name:     "some-get-plan",
							Resource: "some-get-plan",
							Passed:   []string{"a", "b"},
							Trigger:  true,
						},
						{
							Name:     "some-other-get-plan",
							Resource: "some-other-get-plan",
							Trigger:  false,
						},
					}))
				})
			})

			Context("when a plan has an ensure hook on a get", func() {
				BeforeEach(func() {
					jobConfig.Plan = PlanSequence{
						{
							Get: "a",
							Ensure: &PlanConfig{
								Get: "b",
							},
						},
					}
				})

				It("returns an input config for all get plans", func() {
					Ω(inputs).Should(ConsistOf(
						JobInput{
							Name:     "a",
							Resource: "a",
						},
						JobInput{
							Name:     "b",
							Resource: "b",
						},
					))
				})
			})

			Context("when a plan has an success hook on a get", func() {
				BeforeEach(func() {
					jobConfig.Plan = PlanSequence{
						{
							Get: "a",
							Success: &PlanConfig{
								Get: "b",
							},
						},
					}
				})

				It("returns an input config for all get plans", func() {
					Ω(inputs).Should(ConsistOf(
						JobInput{
							Name:     "a",
							Resource: "a",
						},
						JobInput{
							Name:     "b",
							Resource: "b",
						},
					))
				})
			})

			Context("when a plan has an failure hook on a get", func() {
				BeforeEach(func() {
					jobConfig.Plan = PlanSequence{
						{
							Get: "a",
							Failure: &PlanConfig{
								Get: "b",
							},
						},
					}
				})

				It("returns an input config for all get plans", func() {
					Ω(inputs).Should(ConsistOf(
						JobInput{
							Name:     "a",
							Resource: "a",
						},
						JobInput{
							Name:     "b",
							Resource: "b",
						},
					))
				})
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
							Trigger:  false,
						},
					}))
				})
			})

			Context("when a simple aggregate plan is the first step", func() {
				BeforeEach(func() {
					jobConfig.Plan = PlanSequence{
						{
							Aggregate: &PlanSequence{
								{Get: "a"},
								{Put: "y"},
								{Get: "b", Resource: "some-resource", Passed: []string{"x"}},
								{Get: "c", Trigger: true},
							},
						},
					}
				})

				It("returns an input config for all get plans", func() {
					Ω(inputs).Should(Equal([]JobInput{
						{
							Name:     "a",
							Resource: "a",
							Trigger:  false,
						},
						{
							Name:     "b",
							Resource: "some-resource",
							Passed:   []string{"x"},
							Trigger:  false,
						},
						{
							Name:     "c",
							Resource: "c",
							Trigger:  true,
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
								{Get: "c", Trigger: true},
							},
						},
					}
				})

				It("returns an input config for all of the get plans present", func() {
					Ω(inputs).Should(Equal([]JobInput{
						{
							Name:     "a",
							Resource: "a",
							Trigger:  false,
						},
						{
							Name:     "b",
							Resource: "some-resource",
							Passed:   []string{"x"},
							Trigger:  false,
						},
						{
							Name:     "c",
							Resource: "c",
							Trigger:  true,
						},
					}))
				})
			})

			Context("when there are not gets in the plan", func() {
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

			Context("when a plan has an ensure on a put", func() {
				BeforeEach(func() {
					jobConfig.Plan = PlanSequence{
						{
							Put: "a",
							Ensure: &PlanConfig{
								Put: "b",
							},
						},
					}
				})

				It("returns an output config for all put plans", func() {
					Ω(outputs).Should(ConsistOf(
						JobOutput{
							Name:     "a",
							Resource: "a",
						},
						JobOutput{
							Name:     "b",
							Resource: "b",
						},
					))
				})
			})

			Context("when a plan has an success hook on a put", func() {
				BeforeEach(func() {
					jobConfig.Plan = PlanSequence{
						{
							Put: "a",
							Success: &PlanConfig{
								Put: "b",
							},
						},
					}
				})

				It("returns an output config for all put plans", func() {
					Ω(outputs).Should(ConsistOf(
						JobOutput{
							Name:     "a",
							Resource: "a",
						},
						JobOutput{
							Name:     "b",
							Resource: "b",
						},
					))
				})
			})

			Context("when a plan has an failure hook on a put", func() {
				BeforeEach(func() {
					jobConfig.Plan = PlanSequence{
						{
							Put: "a",
							Failure: &PlanConfig{
								Put: "b",
							},
						},
					}
				})

				It("returns an output config for all put plans", func() {
					Ω(outputs).Should(ConsistOf(
						JobOutput{
							Name:     "a",
							Resource: "a",
						},
						JobOutput{
							Name:     "b",
							Resource: "b",
						},
					))
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
