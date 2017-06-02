package plan_test

import (
	"github.com/concourse/atc"
	"github.com/concourse/atc/plan"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Planner", func() {
	var planner plan.Planner

	Describe("GenerateCheckPlanForJob", func() {
		Context("when there is a get step with base resource type", func() {
			It("returns plan with check step", func() {
				checkPlan, err := planner.GenerateCheckPlanForJob(atc.Config{
					Resources: atc.ResourceConfigs{
						{
							Name:   "some-resource",
							Type:   "git",
							Source: atc.Source{"some": "source"},
						},
					},
					Jobs: atc.JobConfigs{
						{
							Name: "some-job",
							Plan: atc.PlanSequence{
								{
									Get:    "some-resource",
									Params: atc.Params{"some": "params"},
								},
							},
						},
					},
				}, "some-job")
				Expect(err).NotTo(HaveOccurred())
				Expect(checkPlan.Steps).To(ConsistOf(
					plan.CheckStep{
						Actions: []interface{}{
							plan.CheckAction{
								RootFSSource: plan.BaseResourceTypeRootFSSource{
									Name: "git",
								},
								Source: atc.Source{"some": "source"},
							},
						},
					},
				))
			})
		})

		Context("when there is a get step with custom resource type", func() {
			It("returns plan with check step with actions to check resource types, get resource types and check for resource", func() {
				checkPlan, err := planner.GenerateCheckPlanForJob(atc.Config{
					ResourceTypes: atc.ResourceTypes{
						{
							Name:   "some-resource-type-type",
							Type:   "git",
							Source: atc.Source{"some-resource-type-type": "source"},
						},
						{
							Name:   "some-resource-type",
							Type:   "some-resource-type-type",
							Source: atc.Source{"some-resource-type": "source"},
						},
					},
					Resources: atc.ResourceConfigs{
						{
							Name:   "some-resource",
							Type:   "some-resource-type",
							Source: atc.Source{"some-resource": "source"},
						},
					},
					Jobs: atc.JobConfigs{
						{
							Name: "some-job",
							Plan: atc.PlanSequence{
								{
									Get:    "some-resource",
									Params: atc.Params{"some": "params"},
								},
							},
						},
					},
				}, "some-job")
				Expect(err).NotTo(HaveOccurred())
				Expect(checkPlan.Steps).To(HaveLen(1))
				checkStep, ok := checkPlan.Steps[0].(plan.CheckStep)
				Expect(ok).To(BeTrue())
				Expect(checkStep.Actions).To(HaveLen(5))
				Expect(checkStep.Actions[0]).To(Equal(
					plan.CheckAction{
						RootFSSource: plan.BaseResourceTypeRootFSSource{
							Name: "git",
						},
						Source: atc.Source{"some-resource-type-type": "source"},
					}))

				getAction, ok := checkStep.Actions[1].(plan.GetAction)
				Expect(ok).To(BeTrue())
				Expect(getAction.RootFSSource).To(Equal(plan.BaseResourceTypeRootFSSource{
					Name: "git",
				}))
				Expect(getAction.Source).To(Equal(atc.Source{"some-resource-type-type": "source"}))
				Expect(getAction.Outputs).To(HaveLen(1))

				Expect(checkStep.Actions[2]).To(Equal(
					plan.CheckAction{
						RootFSSource: plan.OutputRootFSSource{
							Name: getAction.Outputs[0].Name,
						},
						Source: atc.Source{"some-resource-type": "source"},
					}))

				getAction2, ok := checkStep.Actions[3].(plan.GetAction)
				Expect(ok).To(BeTrue())
				Expect(getAction2.RootFSSource).To(Equal(plan.OutputRootFSSource{
					Name: getAction.Outputs[0].Name,
				}))
				Expect(getAction2.Source).To(Equal(atc.Source{"some-resource-type": "source"}))
				Expect(getAction2.Outputs).To(HaveLen(1))

				Expect(checkStep.Actions[4]).To(Equal(
					plan.CheckAction{
						RootFSSource: plan.OutputRootFSSource{
							Name: getAction2.Outputs[0].Name,
						},
						Source: atc.Source{"some-resource": "source"},
					}))
			})
		})

		Context("when there is a do step with a get step", func() {
			It("returns plan with check step", func() {
				checkPlan, err := planner.GenerateCheckPlanForJob(atc.Config{
					Resources: atc.ResourceConfigs{
						{
							Name:   "some-resource",
							Type:   "git",
							Source: atc.Source{"some": "source"},
						},
					},
					Jobs: atc.JobConfigs{
						{
							Name: "some-job",
							Plan: atc.PlanSequence{
								{
									Do: &atc.PlanSequence{
										{
											Get:    "some-resource",
											Params: atc.Params{"some": "params"},
										},
									},
								},
							},
						},
					},
				}, "some-job")
				Expect(err).NotTo(HaveOccurred())
				Expect(checkPlan.Steps).To(ConsistOf(
					plan.CheckStep{
						Actions: []interface{}{
							plan.CheckAction{
								RootFSSource: plan.BaseResourceTypeRootFSSource{
									Name: "git",
								},
								Source: atc.Source{"some": "source"},
							},
						},
					},
				))
			})
		})

		Context("when there is an aggregate step with a get step", func() {
			It("returns plan with check step", func() {
				checkPlan, err := planner.GenerateCheckPlanForJob(atc.Config{
					Resources: atc.ResourceConfigs{
						{
							Name:   "some-resource",
							Type:   "git",
							Source: atc.Source{"some": "source"},
						},
					},
					Jobs: atc.JobConfigs{
						{
							Name: "some-job",
							Plan: atc.PlanSequence{
								{
									Aggregate: &atc.PlanSequence{
										{
											Get:    "some-resource",
											Params: atc.Params{"some": "params"},
										},
										{
											Get:    "some-resource",
											Params: atc.Params{"some": "other-params"},
										},
									},
								},
							},
						},
					},
				}, "some-job")
				Expect(err).NotTo(HaveOccurred())
				Expect(checkPlan.Steps).To(ConsistOf(
					plan.CheckStep{
						Actions: []interface{}{
							plan.CheckAction{
								RootFSSource: plan.BaseResourceTypeRootFSSource{
									Name: "git",
								},
								Source: atc.Source{"some": "source"},
							},
						},
					},
					plan.CheckStep{
						Actions: []interface{}{
							plan.CheckAction{
								RootFSSource: plan.BaseResourceTypeRootFSSource{
									Name: "git",
								},
								Source: atc.Source{"some": "source"},
							},
						},
					},
				))
			})
		})

		Context("when resource is not in resources list", func() {
			It("returns an error", func() {
				_, err := planner.GenerateCheckPlanForJob(atc.Config{
					Resources: atc.ResourceConfigs{},
					Jobs: atc.JobConfigs{
						{
							Name: "some-job",
							Plan: atc.PlanSequence{
								{
									Get:    "some-resource",
									Params: atc.Params{"some": "params"},
								},
							},
						},
					},
				}, "some-job")
				Expect(err).To(HaveOccurred())
			})
		})

		Context("when there are no checks", func() {
			It("returns empty plan", func() {
				checkPlan, err := planner.GenerateCheckPlanForJob(atc.Config{
					Resources: atc.ResourceConfigs{
						{
							Name: "some-resource",
							Type: "git",
						},
					},
					Jobs: atc.JobConfigs{
						{
							Name: "some-job",
							Plan: atc.PlanSequence{
								{
									Put: "some-resource",
								},
							},
						},
					},
				}, "some-job")
				Expect(err).NotTo(HaveOccurred())
				Expect(checkPlan.Steps).To(BeEmpty())
			})
		})

		Context("when job name is not in pipeline config", func() {
			It("returns an error", func() {
				_, err := planner.GenerateCheckPlanForJob(atc.Config{}, "non-existent-job")
				Expect(err).To(HaveOccurred())
			})
		})
	})
})
