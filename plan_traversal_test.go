package atc_test

import (
	"github.com/concourse/atc"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("PlanTraversal", func() {
	Describe("Traverse", func() {
		It("calls traverse function on every plan in the plan tree", func() {
			allPlans := []*atc.Plan{}

			traverseFunc := func(plan *atc.Plan) {
				allPlans = append(allPlans, plan)
			}

			planTraversal := atc.PlanTraversal{
				TraverseFunc: traverseFunc,
			}

			plan := &atc.Plan{
				ID: "0",
				Aggregate: &atc.AggregatePlan{
					atc.Plan{
						ID: "1",
						Aggregate: &atc.AggregatePlan{
							atc.Plan{
								ID: "2",
								Task: &atc.TaskPlan{
									Name: "name",
								},
							},
						},
					},

					atc.Plan{
						ID: "3",
						Get: &atc.GetPlan{
							Name: "name",
						},
					},

					atc.Plan{
						ID: "4",
						Put: &atc.PutPlan{
							Name: "name",
						},
					},

					atc.Plan{
						ID: "5",
						Task: &atc.TaskPlan{
							Name: "name",
						},
					},

					atc.Plan{
						ID: "6",
						Ensure: &atc.EnsurePlan{
							Step: atc.Plan{
								ID: "7",
								Task: &atc.TaskPlan{
									Name: "name",
								},
							},
							Next: atc.Plan{
								ID: "8",
								Task: &atc.TaskPlan{
									Name: "name",
								},
							},
						},
					},

					atc.Plan{
						ID: "9",
						OnSuccess: &atc.OnSuccessPlan{
							Step: atc.Plan{
								ID: "10",
								Task: &atc.TaskPlan{
									Name: "name",
								},
							},
							Next: atc.Plan{
								ID: "11",
								Task: &atc.TaskPlan{
									Name: "name",
								},
							},
						},
					},

					atc.Plan{
						ID: "12",
						OnFailure: &atc.OnFailurePlan{
							Step: atc.Plan{
								ID: "13",
								Task: &atc.TaskPlan{
									Name: "name",
								},
							},
							Next: atc.Plan{
								ID: "14",
								Task: &atc.TaskPlan{
									Name: "name",
								},
							},
						},
					},

					atc.Plan{
						ID: "15",
						Try: &atc.TryPlan{
							Step: atc.Plan{
								ID: "16",
								Task: &atc.TaskPlan{
									Name: "name",
								},
							},
						},
					},

					atc.Plan{
						ID: "17",
						DependentGet: &atc.DependentGetPlan{
							Name: "name",
						},
					},

					atc.Plan{
						ID: "18",
						Timeout: &atc.TimeoutPlan{
							Step: atc.Plan{
								ID: "19",
								Task: &atc.TaskPlan{
									Name: "name",
								},
							},
							Duration: "lol",
						},
					},

					atc.Plan{
						ID: "20",
						Do: &atc.DoPlan{
							atc.Plan{
								ID: "21",
								Task: &atc.TaskPlan{
									Name: "name",
								},
							},
						},
					},

					atc.Plan{
						ID: "22",
						Retry: &atc.RetryPlan{
							atc.Plan{
								ID: "23",
								Task: &atc.TaskPlan{
									Name: "name",
								},
							},
							atc.Plan{
								ID: "24",
								Task: &atc.TaskPlan{
									Name: "name",
								},
							},
							atc.Plan{
								ID: "25",
								Task: &atc.TaskPlan{
									Name: "name",
								},
							},
						},
					},
				},
			}

			planTraversal.Traverse(plan)

			Expect(allPlans[0]).To(Equal(plan))
			Expect(allPlans[1]).To(Equal(&(*plan.Aggregate)[0]))
			Expect(allPlans[2]).To(Equal(&(*(*plan.Aggregate)[0].Aggregate)[0]))
			Expect(allPlans[3]).To(Equal(&(*plan.Aggregate)[1]))
			Expect(allPlans[4]).To(Equal(&(*plan.Aggregate)[2]))
			Expect(allPlans[5]).To(Equal(&(*plan.Aggregate)[3]))
			Expect(allPlans[6]).To(Equal(&(*plan.Aggregate)[4]))
			Expect(allPlans[7]).To(Equal(&(*plan.Aggregate)[4].Ensure.Step))
			Expect(allPlans[8]).To(Equal(&(*plan.Aggregate)[4].Ensure.Next))
			Expect(allPlans[9]).To(Equal(&(*plan.Aggregate)[5]))
			Expect(allPlans[10]).To(Equal(&(*plan.Aggregate)[5].OnSuccess.Step))
			Expect(allPlans[11]).To(Equal(&(*plan.Aggregate)[5].OnSuccess.Next))
			Expect(allPlans[12]).To(Equal(&(*plan.Aggregate)[6]))
			Expect(allPlans[13]).To(Equal(&(*plan.Aggregate)[6].OnFailure.Step))
			Expect(allPlans[14]).To(Equal(&(*plan.Aggregate)[6].OnFailure.Next))
			Expect(allPlans[15]).To(Equal(&(*plan.Aggregate)[7]))
			Expect(allPlans[16]).To(Equal(&(*plan.Aggregate)[7].Try.Step))
			Expect(allPlans[17]).To(Equal(&(*plan.Aggregate)[8]))
			Expect(allPlans[18]).To(Equal(&(*plan.Aggregate)[9]))
			Expect(allPlans[19]).To(Equal(&(*plan.Aggregate)[9].Timeout.Step))
			Expect(allPlans[20]).To(Equal(&(*plan.Aggregate)[10]))
			Expect(allPlans[21]).To(Equal(&(*(*plan.Aggregate)[10].Do)[0]))
			Expect(allPlans[22]).To(Equal(&(*plan.Aggregate)[11]))
			Expect(allPlans[23]).To(Equal(&(*(*plan.Aggregate)[11].Retry)[0]))
			Expect(allPlans[24]).To(Equal(&(*(*plan.Aggregate)[11].Retry)[1]))
			Expect(allPlans[25]).To(Equal(&(*(*plan.Aggregate)[11].Retry)[2]))
		})
	})
})
