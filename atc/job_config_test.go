package atc_test

import (
	"github.com/concourse/concourse/atc"

	. "github.com/onsi/ginkgo/v2"
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

	Describe("Inputs", func() {
		var (
			jobConfig atc.JobConfig

			inputs []atc.JobInputParams
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
					jobConfig.PlanSequence = []atc.Step{}
				})

				It("returns an empty set of inputs", func() {
					Expect(inputs).To(BeEmpty())
				})
			})

			Context("with two serial gets", func() {
				BeforeEach(func() {
					jobConfig.PlanSequence = []atc.Step{
						{
							Config: &atc.GetStep{
								Name:    "some-get-plan",
								Passed:  []string{"a", "b"},
								Trigger: true,
							},
						},
						{
							Config: &atc.GetStep{
								Name: "some-other-get-plan",
							},
						},
					}
				})

				It("uses both for inputs", func() {
					Expect(inputs).To(Equal([]atc.JobInputParams{
						{
							JobInput: atc.JobInput{
								Name:     "some-get-plan",
								Resource: "some-get-plan",
								Passed:   []string{"a", "b"},
								Trigger:  true,
							},
						},
						{
							JobInput: atc.JobInput{
								Name:     "some-other-get-plan",
								Resource: "some-other-get-plan",
								Trigger:  false,
							},
						},
					}))

				})
			})

			Context("when a plan has a version on a get", func() {
				BeforeEach(func() {
					jobConfig.PlanSequence = []atc.Step{
						{
							Config: &atc.GetStep{
								Name: "a",
								Version: &atc.VersionConfig{
									Every: true,
								},
							},
						},
					}
				})

				It("returns an input config with the version", func() {
					Expect(inputs).To(Equal(
						[]atc.JobInputParams{
							{
								JobInput: atc.JobInput{
									Name:     "a",
									Resource: "a",
									Version: &atc.VersionConfig{
										Every: true,
									},
								},
							},
						},
					))
				})
			})

			Context("when a job has an ensure hook", func() {
				BeforeEach(func() {
					jobConfig.PlanSequence = []atc.Step{
						{
							Config: &atc.GetStep{
								Name: "a",
							},
						},
					}

					jobConfig.Ensure = &atc.Step{
						Config: &atc.GetStep{
							Name: "b",
						},
					}
				})

				It("returns an input config for all get plans", func() {
					Expect(inputs).To(ConsistOf(
						atc.JobInputParams{
							JobInput: atc.JobInput{
								Name:     "a",
								Resource: "a",
							},
						},
						atc.JobInputParams{
							JobInput: atc.JobInput{
								Name:     "b",
								Resource: "b",
							},
						},
					))
				})
			})

			Context("when a job has a success hook", func() {
				BeforeEach(func() {
					jobConfig.PlanSequence = []atc.Step{
						{
							Config: &atc.GetStep{
								Name: "a",
							},
						},
					}

					jobConfig.OnSuccess = &atc.Step{
						Config: &atc.GetStep{
							Name: "b",
						},
					}
				})

				It("returns an input config for all get plans", func() {
					Expect(inputs).To(ConsistOf(
						atc.JobInputParams{
							JobInput: atc.JobInput{
								Name:     "a",
								Resource: "a",
							},
						},
						atc.JobInputParams{
							JobInput: atc.JobInput{
								Name:     "b",
								Resource: "b",
							},
						},
					))

				})
			})

			Context("when a job has a failure hook", func() {
				BeforeEach(func() {
					jobConfig.PlanSequence = []atc.Step{
						{
							Config: &atc.GetStep{
								Name: "a",
							},
						},
					}

					jobConfig.OnFailure = &atc.Step{
						Config: &atc.GetStep{
							Name: "b",
						},
					}
				})

				It("returns an input config for all get plans", func() {
					Expect(inputs).To(ConsistOf(
						atc.JobInputParams{
							JobInput: atc.JobInput{
								Name:     "a",
								Resource: "a",
							},
						},
						atc.JobInputParams{
							JobInput: atc.JobInput{
								Name:     "b",
								Resource: "b",
							},
						},
					))

				})
			})

			Context("when a job has an abort hook", func() {
				BeforeEach(func() {
					jobConfig.PlanSequence = []atc.Step{
						{
							Config: &atc.GetStep{
								Name: "a",
							},
						},
					}

					jobConfig.OnAbort = &atc.Step{
						Config: &atc.GetStep{
							Name: "b",
						},
					}
				})

				It("returns an input config for all get plans", func() {
					Expect(inputs).To(ConsistOf(
						atc.JobInputParams{
							JobInput: atc.JobInput{
								Name:     "a",
								Resource: "a",
							},
						},
						atc.JobInputParams{
							JobInput: atc.JobInput{
								Name:     "b",
								Resource: "b",
							},
						},
					))

				})
			})

			Context("when a job has an error hook", func() {
				BeforeEach(func() {
					jobConfig.PlanSequence = []atc.Step{
						{
							Config: &atc.GetStep{
								Name: "a",
							},
						},
					}

					jobConfig.OnError = &atc.Step{
						Config: &atc.GetStep{
							Name: "b",
						},
					}
				})

				It("returns an input config for all get plans", func() {
					Expect(inputs).To(ConsistOf(
						atc.JobInputParams{
							JobInput: atc.JobInput{
								Name:     "a",
								Resource: "a",
							},
						},
						atc.JobInputParams{
							JobInput: atc.JobInput{
								Name:     "b",
								Resource: "b",
							},
						},
					))

				})
			})

			Context("when a resource is specified", func() {
				BeforeEach(func() {
					jobConfig.PlanSequence = []atc.Step{
						{
							Config: &atc.GetStep{
								Name:     "some-get-plan",
								Resource: "some-get-resource",
							},
						},
					}
				})

				It("uses it as resource in the input config", func() {
					Expect(inputs).To(Equal([]atc.JobInputParams{
						{
							JobInput: atc.JobInput{
								Name:     "some-get-plan",
								Resource: "some-get-resource",
								Trigger:  false,
							},
						},
					}))

				})
			})

			Context("when a simple in_parallel plan is the first step", func() {
				BeforeEach(func() {
					jobConfig.PlanSequence = []atc.Step{
						{
							Config: &atc.InParallelStep{
								Config: atc.InParallelConfig{
									Steps: []atc.Step{
										{
											Config: &atc.GetStep{
												Name: "a",
											},
										},
										{
											Config: &atc.PutStep{
												Name: "y",
											},
										},
										{
											Config: &atc.GetStep{
												Name:     "b",
												Resource: "some-resource", Passed: []string{"x"},
											},
										},
										{
											Config: &atc.GetStep{
												Name: "c", Trigger: true,
											},
										},
									},
								},
							},
						},
					}
				})

				It("returns an input config for all get plans", func() {
					Expect(inputs).To(Equal([]atc.JobInputParams{
						{
							JobInput: atc.JobInput{
								Name:     "a",
								Resource: "a",
								Trigger:  false,
							},
						},
						{
							JobInput: atc.JobInput{
								Name:     "b",
								Resource: "some-resource",
								Passed:   []string{"x"},
								Trigger:  false,
							},
						},
						{
							JobInput: atc.JobInput{
								Name:     "c",
								Resource: "c",
								Trigger:  true,
							},
						},
					}))

				})
			})

			Context("when there are no gets in the plan", func() {
				BeforeEach(func() {
					jobConfig.PlanSequence = []atc.Step{
						{
							Config: &atc.PutStep{
								Name: "some-put-plan",
							},
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
					jobConfig.PlanSequence = []atc.Step{}
				})

				It("returns an empty set of outputs", func() {
					Expect(outputs).To(BeEmpty())
				})
			})

			Context("when a simple plan is configured", func() {
				BeforeEach(func() {
					jobConfig.PlanSequence = []atc.Step{
						{
							Config: &atc.PutStep{
								Name:     "some-name",
								Resource: "some-resource",
							},
						},
					}
				})

				It("returns an output for all of the put plans present", func() {
					Expect(outputs).To(Equal([]atc.JobOutput{
						{
							Name:     "some-name",
							Resource: "some-resource",
						},
					}))

				})
			})
		})

		Context("when a job has an ensure hook", func() {
			BeforeEach(func() {
				jobConfig.PlanSequence = []atc.Step{
					{
						Config: &atc.PutStep{
							Name: "a",
						},
					},
				}

				jobConfig.Ensure = &atc.Step{
					Config: &atc.PutStep{
						Name: "b",
					},
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
				jobConfig.PlanSequence = []atc.Step{
					{
						Config: &atc.PutStep{
							Name: "a",
						},
					},
				}

				jobConfig.OnSuccess = &atc.Step{
					Config: &atc.PutStep{
						Name: "b",
					},
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
				jobConfig.PlanSequence = []atc.Step{
					{
						Config: &atc.PutStep{
							Name: "a",
						},
					},
				}

				jobConfig.OnFailure = &atc.Step{
					Config: &atc.PutStep{
						Name: "b",
					},
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
				jobConfig.PlanSequence = []atc.Step{
					{
						Config: &atc.PutStep{
							Name: "a",
						},
					},
				}

				jobConfig.OnAbort = &atc.Step{
					Config: &atc.PutStep{
						Name: "b",
					},
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

		Context("when a job has an error hook", func() {
			BeforeEach(func() {
				jobConfig.PlanSequence = []atc.Step{
					{
						Config: &atc.PutStep{
							Name: "a",
						},
					},
				}

				jobConfig.OnError = &atc.Step{
					Config: &atc.PutStep{
						Name: "b",
					},
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

		Context("when the plan contains no puts steps", func() {
			BeforeEach(func() {
				jobConfig.PlanSequence = []atc.Step{
					{
						Config: &atc.GetStep{
							Name: "some-put-plan",
						},
					},
				}
			})

			It("returns an empty set of outputs", func() {
				Expect(outputs).To(BeEmpty())
			})
		})
	})
})
