package engine_test

import (
	"code.cloudfoundry.org/lager/lagertest"
	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/db/dbfakes"
	"github.com/concourse/atc/engine"
	"github.com/concourse/atc/engine/enginefakes"
	"github.com/concourse/atc/exec/execfakes"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Exec Engine With Hooks", func() {
	var (
		fakeFactory         *execfakes.FakeFactory
		fakeDelegateFactory *enginefakes.FakeBuildDelegateFactory

		execEngine engine.Engine

		build              *dbfakes.FakeBuild
		expectedTeamID     = 1111
		expectedPipelineID = 2222
		expectedJobID      = 3333
		expectedBuildID    = 4444
		expectedMetadata   engine.StepMetadata
		logger             *lagertest.TestLogger

		fakeDelegate *enginefakes.FakeBuildDelegate
	)

	BeforeEach(func() {
		logger = lagertest.NewTestLogger("test")

		fakeFactory = new(execfakes.FakeFactory)
		fakeDelegateFactory = new(enginefakes.FakeBuildDelegateFactory)

		execEngine = engine.NewExecEngine(
			fakeFactory,
			fakeDelegateFactory,
			"http://example.com",
		)

		fakeDelegate = new(enginefakes.FakeBuildDelegate)
		fakeDelegateFactory.DelegateReturns(fakeDelegate)

		build = new(dbfakes.FakeBuild)
		build.IDReturns(expectedBuildID)
		build.NameReturns("42")
		build.JobNameReturns("some-job")
		build.JobIDReturns(expectedJobID)
		build.PipelineNameReturns("some-pipeline")
		build.PipelineIDReturns(expectedPipelineID)
		build.TeamNameReturns("some-team")
		build.TeamIDReturns(expectedTeamID)

		expectedMetadata = engine.StepMetadata{
			BuildID:      expectedBuildID,
			BuildName:    "42",
			JobName:      "some-job",
			PipelineName: "some-pipeline",
			TeamName:     "some-team",
			ExternalURL:  "http://example.com",
		}
	})

	Context("running hooked composes", func() {
		var (
			taskStep   *execfakes.FakeStep
			inputStep  *execfakes.FakeStep
			outputStep *execfakes.FakeStep
		)

		BeforeEach(func() {
			taskStep = new(execfakes.FakeStep)
			taskStep.SucceededReturns(true)
			fakeFactory.TaskReturns(taskStep)

			inputStep = new(execfakes.FakeStep)
			inputStep.SucceededReturns(true)
			fakeFactory.GetReturns(inputStep)

			outputStep = new(execfakes.FakeStep)
			outputStep.SucceededReturns(true)
			fakeFactory.PutReturns(outputStep)

			taskStep.RunReturns(nil)
			inputStep.RunReturns(nil)
			outputStep.RunReturns(nil)
		})

		Context("constructing steps", func() {
			var (
				fakeDelegate *enginefakes.FakeBuildDelegate
			)

			BeforeEach(func() {
				fakeDelegate = new(enginefakes.FakeBuildDelegate)
				fakeDelegateFactory.DelegateReturns(fakeDelegate)
			})

			Context("with all the hooks", func() {
				var (
					plan               atc.Plan
					inputPlan          atc.Plan
					failureTaskPlan    atc.Plan
					successTaskPlan    atc.Plan
					completionTaskPlan atc.Plan
					nextTaskPlan       atc.Plan
					planFactory        atc.PlanFactory
				)

				BeforeEach(func() {
					planFactory = atc.NewPlanFactory(123)
					inputPlan = planFactory.NewPlan(atc.GetPlan{
						Name: "some-input",
					})
					failureTaskPlan = planFactory.NewPlan(atc.TaskPlan{
						Name:   "some-failure-task",
						Config: &atc.TaskConfig{},
					})
					successTaskPlan = planFactory.NewPlan(atc.TaskPlan{
						Name:   "some-success-task",
						Config: &atc.TaskConfig{},
					})
					completionTaskPlan = planFactory.NewPlan(atc.TaskPlan{
						Name:   "some-completion-task",
						Config: &atc.TaskConfig{},
					})
					nextTaskPlan = planFactory.NewPlan(atc.TaskPlan{
						Name:   "some-next-task",
						Config: &atc.TaskConfig{},
					})

					plan = planFactory.NewPlan(atc.OnSuccessPlan{
						Step: planFactory.NewPlan(atc.EnsurePlan{
							Step: planFactory.NewPlan(atc.OnSuccessPlan{
								Step: planFactory.NewPlan(atc.OnFailurePlan{
									Step: inputPlan,
									Next: failureTaskPlan,
								}),
								Next: successTaskPlan,
							}),
							Next: completionTaskPlan,
						}),
						Next: nextTaskPlan,
					})

					build, err := execEngine.CreateBuild(logger, build, plan)
					Expect(err).NotTo(HaveOccurred())
					build.Resume(logger)
				})

				It("constructs the step correctly", func() {
					Expect(fakeFactory.GetCallCount()).To(Equal(1))
					logger, plan, dbBuild, stepMetadata, containerMetadata, _ := fakeFactory.GetArgsForCall(0)
					Expect(logger).NotTo(BeNil())
					Expect(dbBuild).To(Equal(build))
					Expect(plan).To(Equal(inputPlan))
					Expect(stepMetadata).To(Equal(expectedMetadata))
					Expect(containerMetadata).To(Equal(db.ContainerMetadata{
						PipelineID:   expectedPipelineID,
						PipelineName: "some-pipeline",
						JobID:        expectedJobID,
						JobName:      "some-job",
						BuildID:      expectedBuildID,
						BuildName:    "42",
						StepName:     "some-input",
						Type:         db.ContainerTypeGet,
					}))
				})

				It("constructs the completion hook correctly", func() {
					Expect(fakeFactory.TaskCallCount()).To(Equal(4))
					logger, plan, dbBuild, containerMetadata, _ := fakeFactory.TaskArgsForCall(2)
					Expect(logger).NotTo(BeNil())
					Expect(dbBuild).To(Equal(build))
					Expect(plan).To(Equal(completionTaskPlan))
					Expect(containerMetadata).To(Equal(db.ContainerMetadata{
						PipelineID:   expectedPipelineID,
						PipelineName: "some-pipeline",
						JobID:        expectedJobID,
						JobName:      "some-job",
						BuildID:      expectedBuildID,
						BuildName:    "42",
						StepName:     "some-completion-task",
						Type:         db.ContainerTypeTask,
					}))
				})

				It("constructs the failure hook correctly", func() {
					Expect(fakeFactory.TaskCallCount()).To(Equal(4))
					logger, plan, dbBuild, containerMetadata, _ := fakeFactory.TaskArgsForCall(0)
					Expect(logger).NotTo(BeNil())
					Expect(dbBuild).To(Equal(build))
					Expect(plan).To(Equal(failureTaskPlan))
					Expect(containerMetadata).To(Equal(db.ContainerMetadata{
						PipelineID:   expectedPipelineID,
						PipelineName: "some-pipeline",
						JobID:        expectedJobID,
						JobName:      "some-job",
						BuildID:      expectedBuildID,
						BuildName:    "42",
						StepName:     "some-failure-task",
						Type:         db.ContainerTypeTask,
					}))
				})

				It("constructs the success hook correctly", func() {
					Expect(fakeFactory.TaskCallCount()).To(Equal(4))
					logger, plan, dbBuild, containerMetadata, _ := fakeFactory.TaskArgsForCall(1)
					Expect(logger).NotTo(BeNil())
					Expect(dbBuild).To(Equal(build))
					Expect(plan).To(Equal(successTaskPlan))
					Expect(containerMetadata).To(Equal(db.ContainerMetadata{
						PipelineID:   expectedPipelineID,
						PipelineName: "some-pipeline",
						JobID:        expectedJobID,
						JobName:      "some-job",
						BuildID:      expectedBuildID,
						BuildName:    "42",
						StepName:     "some-success-task",
						Type:         db.ContainerTypeTask,
					}))
				})

				It("constructs the next step correctly", func() {
					Expect(fakeFactory.TaskCallCount()).To(Equal(4))
					logger, plan, dbBuild, containerMetadata, _ := fakeFactory.TaskArgsForCall(3)
					Expect(logger).NotTo(BeNil())
					Expect(dbBuild).To(Equal(build))
					Expect(plan).To(Equal(nextTaskPlan))
					Expect(containerMetadata).To(Equal(db.ContainerMetadata{
						PipelineID:   expectedPipelineID,
						PipelineName: "some-pipeline",
						JobID:        expectedJobID,
						JobName:      "some-job",
						BuildID:      expectedBuildID,
						BuildName:    "42",
						StepName:     "some-next-task",
						Type:         db.ContainerTypeTask,
					}))
				})
			})
		})

		Context("when the step succeeds", func() {
			var planFactory atc.PlanFactory

			BeforeEach(func() {
				planFactory = atc.NewPlanFactory(123)
				inputStep.SucceededReturns(true)
			})

			It("runs the next step", func() {
				plan := planFactory.NewPlan(atc.OnSuccessPlan{
					Step: planFactory.NewPlan(atc.GetPlan{
						Name: "some-input",
					}),
					Next: planFactory.NewPlan(atc.TaskPlan{
						Name:   "some-resource",
						Config: &atc.TaskConfig{},
					}),
				})

				build, err := execEngine.CreateBuild(logger, build, plan)

				Expect(err).NotTo(HaveOccurred())

				build.Resume(logger)

				Expect(inputStep.RunCallCount()).To(Equal(1))

				Expect(taskStep.RunCallCount()).To(Equal(1))
			})

			It("runs the success hooks, and completion hooks", func() {
				plan := planFactory.NewPlan(atc.EnsurePlan{
					Step: planFactory.NewPlan(atc.OnSuccessPlan{
						Step: planFactory.NewPlan(atc.GetPlan{
							Name: "some-input",
						}),
						Next: planFactory.NewPlan(atc.TaskPlan{
							Name:   "some-resource",
							Config: &atc.TaskConfig{},
						}),
					}),
					Next: planFactory.NewPlan(atc.OnSuccessPlan{
						Step: planFactory.NewPlan(atc.PutPlan{
							Name: "some-put",
						}),
						Next: planFactory.NewPlan(atc.GetPlan{
							Name: "some-put",
						}),
					}),
				})

				build, err := execEngine.CreateBuild(logger, build, plan)

				Expect(err).NotTo(HaveOccurred())

				build.Resume(logger)

				Expect(inputStep.RunCallCount()).To(Equal(2))

				Expect(taskStep.RunCallCount()).To(Equal(1))

				Expect(outputStep.RunCallCount()).To(Equal(1))
			})

			Context("when the success hook fails, and has a failure hook", func() {
				BeforeEach(func() {
					taskStep.SucceededReturns(false)
				})

				It("does not run the next step", func() {
					plan := planFactory.NewPlan(atc.OnSuccessPlan{
						Step: planFactory.NewPlan(atc.OnSuccessPlan{
							Step: planFactory.NewPlan(atc.GetPlan{
								Name: "some-input",
							}),
							Next: planFactory.NewPlan(atc.OnFailurePlan{
								Step: planFactory.NewPlan(atc.TaskPlan{
									Name:   "some-resource",
									Config: &atc.TaskConfig{},
								}),
								Next: planFactory.NewPlan(atc.TaskPlan{
									Name:   "some-input-success-failure",
									Config: &atc.TaskConfig{},
								}),
							}),
						}),
						Next: planFactory.NewPlan(atc.GetPlan{
							Name: "some-unused-step",
						}),
					})

					build, err := execEngine.CreateBuild(logger, build, plan)

					Expect(err).NotTo(HaveOccurred())

					build.Resume(logger)

					Expect(inputStep.RunCallCount()).To(Equal(1))

					Expect(taskStep.RunCallCount()).To(Equal(2))

					Expect(outputStep.RunCallCount()).To(Equal(0))
				})
			})
		})

		Context("when the step fails", func() {
			var planFactory atc.PlanFactory

			BeforeEach(func() {
				planFactory = atc.NewPlanFactory(123)
				inputStep.SucceededReturns(false)
			})

			It("only run the failure hooks", func() {
				plan := planFactory.NewPlan(atc.OnSuccessPlan{
					Step: planFactory.NewPlan(atc.OnFailurePlan{
						Step: planFactory.NewPlan(atc.GetPlan{
							Name: "some-input",
						}),
						Next: planFactory.NewPlan(atc.TaskPlan{
							Name:   "some-resource",
							Config: &atc.TaskConfig{},
						}),
					}),
					Next: planFactory.NewPlan(atc.GetPlan{
						Name: "some-unused-step",
					}),
				})

				build, err := execEngine.CreateBuild(logger, build, plan)

				Expect(err).NotTo(HaveOccurred())

				build.Resume(logger)

				Expect(inputStep.RunCallCount()).To(Equal(1))

				Expect(taskStep.RunCallCount()).To(Equal(1))

				Expect(outputStep.RunCallCount()).To(Equal(0))

				_, cbErr, successful := fakeDelegate.FinishArgsForCall(0)
				Expect(cbErr).NotTo(HaveOccurred())
				Expect(successful).To(BeFalse())
			})
		})

		Context("when a step in the aggregate fails the step fails", func() {
			var planFactory atc.PlanFactory

			BeforeEach(func() {
				planFactory = atc.NewPlanFactory(123)
				inputStep.SucceededReturns(false)
			})

			It("only run the failure hooks", func() {
				plan := planFactory.NewPlan(atc.OnSuccessPlan{
					Step: planFactory.NewPlan(atc.AggregatePlan{
						planFactory.NewPlan(atc.TaskPlan{
							Name:   "some-resource",
							Config: &atc.TaskConfig{},
						}),
						planFactory.NewPlan(atc.OnFailurePlan{
							Step: planFactory.NewPlan(atc.GetPlan{
								Name: "some-input",
							}),
							Next: planFactory.NewPlan(atc.TaskPlan{
								Name:   "some-resource",
								Config: &atc.TaskConfig{},
							}),
						}),
					}),
					Next: planFactory.NewPlan(atc.GetPlan{
						Name: "some-unused-step",
					}),
				})

				build, err := execEngine.CreateBuild(logger, build, plan)

				Expect(err).NotTo(HaveOccurred())

				build.Resume(logger)

				Expect(inputStep.RunCallCount()).To(Equal(1))

				Expect(taskStep.RunCallCount()).To(Equal(2))

				Expect(outputStep.RunCallCount()).To(Equal(0))
			})
		})
	})
})
