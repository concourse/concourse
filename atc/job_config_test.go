package atc_test

import (
	"github.com/concourse/atc"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("JobConfig", func() {
	Describe("MaxInFlight", func() {
		It("returns the raw MaxInFlight if set", func() {
			jobConfig := atc.JobConfig{
				RawMaxInFlight: 42,
			}

			Expect(jobConfig.MaxInFlight()).To(Equal(42))
		})

		It("returns 1 if Serial is true or SerialGroups has items in it", func() {
			jobConfig := atc.JobConfig{
				Serial:       true,
				SerialGroups: []string{},
			}

			Expect(jobConfig.MaxInFlight()).To(Equal(1))

			jobConfig.SerialGroups = []string{
				"one",
			}
			Expect(jobConfig.MaxInFlight()).To(Equal(1))

			jobConfig.Serial = false
			Expect(jobConfig.MaxInFlight()).To(Equal(1))
		})

		It("returns 1 if Serial is true or SerialGroups has items in it, even if raw MaxInFlight is set", func() {
			jobConfig := atc.JobConfig{
				Serial:         true,
				SerialGroups:   []string{},
				RawMaxInFlight: 3,
			}

			Expect(jobConfig.MaxInFlight()).To(Equal(1))

			jobConfig.SerialGroups = []string{
				"one",
			}
			Expect(jobConfig.MaxInFlight()).To(Equal(1))

			jobConfig.Serial = false
			Expect(jobConfig.MaxInFlight()).To(Equal(1))
		})

		It("returns 0 if MaxInFlight is not set, Serial is false, and SerialGroups is empty", func() {
			jobConfig := atc.JobConfig{
				Serial:       false,
				SerialGroups: []string{},
			}

			Expect(jobConfig.MaxInFlight()).To(Equal(0))
		})
	})

	Describe("GetSerialGroups", func() {
		It("Returns the values if SerialGroups is specified", func() {
			jobConfig := atc.JobConfig{
				SerialGroups: []string{"one", "two"},
			}

			Expect(jobConfig.GetSerialGroups()).To(Equal([]string{"one", "two"}))
		})

		It("Returns the job name if Serial but SerialGroups are not specified", func() {
			jobConfig := atc.JobConfig{
				Name:   "some-job",
				Serial: true,
			}

			Expect(jobConfig.GetSerialGroups()).To(Equal([]string{"some-job"}))
		})

		It("Returns the job name if MaxInFlight but SerialGroups are not specified", func() {
			jobConfig := atc.JobConfig{
				Name:           "some-job",
				RawMaxInFlight: 1,
			}

			Expect(jobConfig.GetSerialGroups()).To(Equal([]string{"some-job"}))
		})

		It("returns an empty slice of strings if there are no groups and it is not serial and has no max-in-flight", func() {
			jobConfig := atc.JobConfig{
				Name:   "some-job",
				Serial: false,
			}

			Expect(jobConfig.GetSerialGroups()).To(Equal([]string{}))
		})
	})

	Describe("Inputs", func() {
		var (
			jobConfig atc.JobConfig

			inputs []atc.JobInput
		)

		BeforeEach(func() {
			jobConfig = atc.JobConfig{}
		})

		JustBeforeEach(func() {
			inputs = jobConfig.Inputs()
		})

		Context("with a build plan", func() {
			Context("with an empty plan", func() {
				BeforeEach(func() {
					jobConfig.Plan = atc.PlanSequence{}
				})

				It("returns an empty set of inputs", func() {
					Expect(inputs).To(BeEmpty())
				})
			})

			Context("with two serial gets", func() {
				BeforeEach(func() {
					jobConfig.Plan = atc.PlanSequence{
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
					Expect(inputs).To(Equal([]atc.JobInput{
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

			Context("when a plan has a version on a get", func() {
				BeforeEach(func() {
					jobConfig.Plan = atc.PlanSequence{
						{
							Get: "a",
							Version: &atc.VersionConfig{
								Every: true,
							},
						},
					}
				})

				It("returns an input config with the version", func() {
					Expect(inputs).To(Equal(
						[]atc.JobInput{
							{
								Name:     "a",
								Resource: "a",
								Version: &atc.VersionConfig{
									Every: true,
								},
							},
						},
					))
				})
			})

			Context("when a job has an ensure hook", func() {
				BeforeEach(func() {
					jobConfig.Plan = atc.PlanSequence{
						{
							Get: "a",
						},
					}

					jobConfig.Ensure = &atc.PlanConfig{
						Get: "b",
					}
				})

				It("returns an input config for all get plans", func() {
					Expect(inputs).To(ConsistOf(
						atc.JobInput{
							Name:     "a",
							Resource: "a",
						},
						atc.JobInput{
							Name:     "b",
							Resource: "b",
						},
					))
				})
			})

			Context("when a job has a success hook", func() {
				BeforeEach(func() {
					jobConfig.Plan = atc.PlanSequence{
						{
							Get: "a",
						},
					}

					jobConfig.Success = &atc.PlanConfig{
						Get: "b",
					}
				})

				It("returns an input config for all get plans", func() {
					Expect(inputs).To(ConsistOf(
						atc.JobInput{
							Name:     "a",
							Resource: "a",
						},
						atc.JobInput{
							Name:     "b",
							Resource: "b",
						},
					))

				})
			})

			Context("when a job has a failure hook", func() {
				BeforeEach(func() {
					jobConfig.Plan = atc.PlanSequence{
						{
							Get: "a",
						},
					}

					jobConfig.Failure = &atc.PlanConfig{
						Get: "b",
					}
				})

				It("returns an input config for all get plans", func() {
					Expect(inputs).To(ConsistOf(
						atc.JobInput{
							Name:     "a",
							Resource: "a",
						},
						atc.JobInput{
							Name:     "b",
							Resource: "b",
						},
					))

				})
			})

			Context("when a job has an abort hook", func() {
				BeforeEach(func() {
					jobConfig.Plan = atc.PlanSequence{
						{
							Get: "a",
						},
					}

					jobConfig.Abort = &atc.PlanConfig{
						Get: "b",
					}
				})

				It("returns an input config for all get plans", func() {
					Expect(inputs).To(ConsistOf(
						atc.JobInput{
							Name:     "a",
							Resource: "a",
						},
						atc.JobInput{
							Name:     "b",
							Resource: "b",
						},
					))

				})
			})

			Context("when a plan has an ensure hook on a get", func() {
				BeforeEach(func() {
					jobConfig.Plan = atc.PlanSequence{
						{
							Get: "a",
							Ensure: &atc.PlanConfig{
								Get: "b",
							},
						},
					}
				})

				It("returns an input config for all get plans", func() {
					Expect(inputs).To(ConsistOf(
						atc.JobInput{
							Name:     "a",
							Resource: "a",
						},
						atc.JobInput{
							Name:     "b",
							Resource: "b",
						},
					))

				})
			})

			Context("when a plan has a success hook on a get", func() {
				BeforeEach(func() {
					jobConfig.Plan = atc.PlanSequence{
						{
							Get: "a",
							Success: &atc.PlanConfig{
								Get: "b",
							},
						},
					}
				})

				It("returns an input config for all get plans", func() {
					Expect(inputs).To(ConsistOf(
						atc.JobInput{
							Name:     "a",
							Resource: "a",
						},
						atc.JobInput{
							Name:     "b",
							Resource: "b",
						},
					))

				})
			})

			Context("when a plan has a failure hook on a get", func() {
				BeforeEach(func() {
					jobConfig.Plan = atc.PlanSequence{
						{
							Get: "a",
							Failure: &atc.PlanConfig{
								Get: "b",
							},
						},
					}
				})

				It("returns an input config for all get plans", func() {
					Expect(inputs).To(ConsistOf(
						atc.JobInput{
							Name:     "a",
							Resource: "a",
						},
						atc.JobInput{
							Name:     "b",
							Resource: "b",
						},
					))

				})
			})

			Context("when a plan has an abort hook on a get", func() {
				BeforeEach(func() {
					jobConfig.Plan = atc.PlanSequence{
						{
							Get: "a",
							Abort: &atc.PlanConfig{
								Get: "b",
							},
						},
					}
				})

				It("returns an input config for all get plans", func() {
					Expect(inputs).To(ConsistOf(
						atc.JobInput{
							Name:     "a",
							Resource: "a",
						},
						atc.JobInput{
							Name:     "b",
							Resource: "b",
						},
					))

				})
			})

			Context("when a resource is specified", func() {
				BeforeEach(func() {
					jobConfig.Plan = atc.PlanSequence{
						{
							Get:      "some-get-plan",
							Resource: "some-get-resource",
						},
					}
				})

				It("uses it as resource in the input config", func() {
					Expect(inputs).To(Equal([]atc.JobInput{
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
					jobConfig.Plan = atc.PlanSequence{
						{
							Aggregate: &atc.PlanSequence{
								{Get: "a"},
								{Put: "y"},
								{Get: "b", Resource: "some-resource", Passed: []string{"x"}},
								{Get: "c", Trigger: true},
							},
						},
					}
				})

				It("returns an input config for all get plans", func() {
					Expect(inputs).To(Equal([]atc.JobInput{
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
					jobConfig.Plan = atc.PlanSequence{
						{
							Aggregate: &atc.PlanSequence{
								{
									Aggregate: &atc.PlanSequence{
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
					Expect(inputs).To(Equal([]atc.JobInput{
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
					jobConfig.Plan = atc.PlanSequence{
						{
							Put: "some-put-plan",
						},
					}
				})

				It("returns an empty set of inputs", func() {
					Expect(inputs).To(BeEmpty())
				})
			})
		})
	})

	Describe("Outputs", func() {
		var (
			jobConfig atc.JobConfig

			outputs []atc.JobOutput
		)

		BeforeEach(func() {
			jobConfig = atc.JobConfig{}
		})

		JustBeforeEach(func() {
			outputs = jobConfig.Outputs()
		})

		Context("with a build plan", func() {
			Context("with an empty plan", func() {
				BeforeEach(func() {
					jobConfig.Plan = atc.PlanSequence{}
				})

				It("returns an empty set of outputs", func() {
					Expect(outputs).To(BeEmpty())
				})
			})

			Context("when an overly complicated plan is configured", func() {
				BeforeEach(func() {
					jobConfig.Plan = atc.PlanSequence{
						{
							Aggregate: &atc.PlanSequence{
								{
									Aggregate: &atc.PlanSequence{
										{Put: "a"},
									},
								},
								{Put: "b", Resource: "some-resource"},
								{
									Do: &atc.PlanSequence{
										{Put: "c"},
									},
								},
							},
						},
					}
				})

				It("returns an output for all of the put plans present", func() {
					Expect(outputs).To(Equal([]atc.JobOutput{
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

			Context("when a job has an ensure hook", func() {
				BeforeEach(func() {
					jobConfig.Plan = atc.PlanSequence{
						{
							Put: "a",
						},
					}

					jobConfig.Ensure = &atc.PlanConfig{
						Put: "b",
					}
				})

				It("returns an input config for all get plans", func() {
					Expect(outputs).To(ConsistOf(
						atc.JobOutput{
							Name:     "a",
							Resource: "a",
						},
						atc.JobOutput{
							Name:     "b",
							Resource: "b",
						},
					))
				})
			})

			Context("when a job has a success hook", func() {
				BeforeEach(func() {
					jobConfig.Plan = atc.PlanSequence{
						{
							Put: "a",
						},
					}

					jobConfig.Success = &atc.PlanConfig{
						Put: "b",
					}
				})

				It("returns an input config for all get plans", func() {
					Expect(outputs).To(ConsistOf(
						atc.JobOutput{
							Name:     "a",
							Resource: "a",
						},
						atc.JobOutput{
							Name:     "b",
							Resource: "b",
						},
					))

				})
			})

			Context("when a job has a failure hook", func() {
				BeforeEach(func() {
					jobConfig.Plan = atc.PlanSequence{
						{
							Put: "a",
						},
					}

					jobConfig.Failure = &atc.PlanConfig{
						Put: "b",
					}
				})

				It("returns an input config for all get plans", func() {
					Expect(outputs).To(ConsistOf(
						atc.JobOutput{
							Name:     "a",
							Resource: "a",
						},
						atc.JobOutput{
							Name:     "b",
							Resource: "b",
						},
					))

				})
			})

			Context("when a job has an abort hook", func() {
				BeforeEach(func() {
					jobConfig.Plan = atc.PlanSequence{
						{
							Put: "a",
						},
					}

					jobConfig.Abort = &atc.PlanConfig{
						Put: "b",
					}
				})

				It("returns an input config for all get plans", func() {
					Expect(outputs).To(ConsistOf(
						atc.JobOutput{
							Name:     "a",
							Resource: "a",
						},
						atc.JobOutput{
							Name:     "b",
							Resource: "b",
						},
					))

				})
			})

			Context("when a plan has an ensure on a put", func() {
				BeforeEach(func() {
					jobConfig.Plan = atc.PlanSequence{
						{
							Put: "a",
							Ensure: &atc.PlanConfig{
								Put: "b",
							},
						},
					}
				})

				It("returns an output config for all put plans", func() {
					Expect(outputs).To(ConsistOf(
						atc.JobOutput{
							Name:     "a",
							Resource: "a",
						},
						atc.JobOutput{
							Name:     "b",
							Resource: "b",
						},
					))

				})
			})

			Context("when a plan has a success hook on a put", func() {
				BeforeEach(func() {
					jobConfig.Plan = atc.PlanSequence{
						{
							Put: "a",
							Success: &atc.PlanConfig{
								Put: "b",
							},
						},
					}
				})

				It("returns an output config for all put plans", func() {
					Expect(outputs).To(ConsistOf(
						atc.JobOutput{
							Name:     "a",
							Resource: "a",
						},
						atc.JobOutput{
							Name:     "b",
							Resource: "b",
						},
					))

				})
			})

			Context("when a plan has a failure hook on a put", func() {
				BeforeEach(func() {
					jobConfig.Plan = atc.PlanSequence{
						{
							Put: "a",
							Failure: &atc.PlanConfig{
								Put: "b",
							},
						},
					}
				})

				It("returns an output config for all put plans", func() {
					Expect(outputs).To(ConsistOf(
						atc.JobOutput{
							Name:     "a",
							Resource: "a",
						},
						atc.JobOutput{
							Name:     "b",
							Resource: "b",
						},
					))

				})
			})

			Context("when a plan has an abort hook on a put", func() {
				BeforeEach(func() {
					jobConfig.Plan = atc.PlanSequence{
						{
							Put: "a",
							Abort: &atc.PlanConfig{
								Put: "b",
							},
						},
					}
				})

				It("returns an output config for all put plans", func() {
					Expect(outputs).To(ConsistOf(
						atc.JobOutput{
							Name:     "a",
							Resource: "a",
						},
						atc.JobOutput{
							Name:     "b",
							Resource: "b",
						},
					))

				})
			})

			Context("when the plan contains no puts steps", func() {
				BeforeEach(func() {
					jobConfig.Plan = atc.PlanSequence{
						{
							Get: "some-put-plan",
						},
					}
				})

				It("returns an empty set of outputs", func() {
					Expect(outputs).To(BeEmpty())
				})
			})
		})
	})
})
