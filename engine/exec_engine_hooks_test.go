package engine_test

import (
	"code.cloudfoundry.org/lager/lagertest"
	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/db/dbfakes"
	"github.com/concourse/atc/engine"
	"github.com/concourse/atc/engine/enginefakes"
	"github.com/concourse/atc/exec"
	"github.com/concourse/atc/exec/execfakes"
	"github.com/concourse/atc/worker"

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
			taskStepFactory *execfakes.FakeStepFactory
			taskStep        *execfakes.FakeStep

			inputStepFactory *execfakes.FakeStepFactory
			inputStep        *execfakes.FakeStep

			outputStepFactory *execfakes.FakeStepFactory
			outputStep        *execfakes.FakeStep

			dependentStepFactory *execfakes.FakeStepFactory
			dependentStep        *execfakes.FakeStep
		)

		BeforeEach(func() {
			taskStepFactory = new(execfakes.FakeStepFactory)
			taskStep = new(execfakes.FakeStep)
			taskStep.ResultStub = successResult(true)
			taskStepFactory.UsingReturns(taskStep)
			fakeFactory.TaskReturns(taskStepFactory)

			inputStepFactory = new(execfakes.FakeStepFactory)
			inputStep = new(execfakes.FakeStep)
			inputStep.ResultStub = successResult(true)
			inputStepFactory.UsingReturns(inputStep)
			fakeFactory.GetReturns(inputStepFactory)

			outputStepFactory = new(execfakes.FakeStepFactory)
			outputStep = new(execfakes.FakeStep)
			outputStep.ResultStub = successResult(true)
			outputStepFactory.UsingReturns(outputStep)
			fakeFactory.PutReturns(outputStepFactory)

			dependentStepFactory = new(execfakes.FakeStepFactory)
			dependentStep = new(execfakes.FakeStep)
			dependentStep.ResultStub = successResult(true)
			dependentStepFactory.UsingReturns(dependentStep)
			fakeFactory.DependentGetReturns(dependentStepFactory)

			taskStep.RunReturns(nil)
			inputStep.RunReturns(nil)
			outputStep.RunReturns(nil)
		})

		Context("constructing steps", func() {
			var (
				fakeDelegate          *enginefakes.FakeBuildDelegate
				fakeInputDelegate     *execfakes.FakeGetDelegate
				fakeExecutionDelegate *execfakes.FakeTaskDelegate
			)

			BeforeEach(func() {
				fakeDelegate = new(enginefakes.FakeBuildDelegate)
				fakeDelegateFactory.DelegateReturns(fakeDelegate)

				fakeInputDelegate = new(execfakes.FakeGetDelegate)
				fakeDelegate.InputDelegateReturns(fakeInputDelegate)

				fakeExecutionDelegate = new(execfakes.FakeTaskDelegate)
				fakeDelegate.ExecutionDelegateReturns(fakeExecutionDelegate)
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
						Config: &atc.LoadTaskConfig{},
					})
					successTaskPlan = planFactory.NewPlan(atc.TaskPlan{
						Name:   "some-success-task",
						Config: &atc.LoadTaskConfig{},
					})
					completionTaskPlan = planFactory.NewPlan(atc.TaskPlan{
						Name:   "some-completion-task",
						Config: &atc.LoadTaskConfig{},
					})
					nextTaskPlan = planFactory.NewPlan(atc.TaskPlan{
						Name:   "some-next-task",
						Config: &atc.LoadTaskConfig{},
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
					logger, teamID, buildID, planID, metadata, sourceName, workerMetadata, delegate, _, _, _, _, _ := fakeFactory.GetArgsForCall(0)
					Expect(logger).NotTo(BeNil())
					Expect(teamID).To(Equal(expectedTeamID))
					Expect(buildID).To(Equal(expectedBuildID))
					Expect(planID).To(Equal(inputPlan.ID))
					Expect(metadata).To(Equal(expectedMetadata))
					Expect(sourceName).To(Equal(worker.ArtifactName("some-input")))
					Expect(workerMetadata).To(Equal(db.ContainerMetadata{
						PipelineID:   expectedPipelineID,
						PipelineName: "some-pipeline",
						JobID:        expectedJobID,
						JobName:      "some-job",
						BuildID:      expectedBuildID,
						BuildName:    "42",
						StepName:     "some-input",
						Type:         db.ContainerTypeGet,
					}))

					Expect(delegate).To(Equal(fakeInputDelegate))
					_, _, location := fakeDelegate.InputDelegateArgsForCall(0)
					Expect(location).NotTo(BeNil())
				})

				It("constructs the completion hook correctly", func() {
					Expect(fakeFactory.TaskCallCount()).To(Equal(4))
					logger, teamID, buildID, planID, sourceName, workerMetadata, delegate, _, _, _, _, _, _, _, _ := fakeFactory.TaskArgsForCall(2)
					Expect(logger).NotTo(BeNil())
					Expect(teamID).To(Equal(expectedTeamID))
					Expect(buildID).To(Equal(expectedBuildID))
					Expect(planID).To(Equal(completionTaskPlan.ID))
					Expect(sourceName).To(Equal(worker.ArtifactName("some-completion-task")))
					Expect(workerMetadata).To(Equal(db.ContainerMetadata{
						PipelineID:   expectedPipelineID,
						PipelineName: "some-pipeline",
						JobID:        expectedJobID,
						JobName:      "some-job",
						BuildID:      expectedBuildID,
						BuildName:    "42",
						StepName:     "some-completion-task",
						Type:         db.ContainerTypeTask,
					}))

					Expect(delegate).To(Equal(fakeExecutionDelegate))

					_, _, location := fakeDelegate.ExecutionDelegateArgsForCall(2)
					Expect(location).NotTo(BeNil())
				})

				It("constructs the failure hook correctly", func() {
					Expect(fakeFactory.TaskCallCount()).To(Equal(4))
					logger, teamID, buildID, planID, sourceName, workerMetadata, delegate, _, _, _, _, _, _, _, _ := fakeFactory.TaskArgsForCall(0)
					Expect(logger).NotTo(BeNil())
					Expect(teamID).To(Equal(expectedTeamID))
					Expect(buildID).To(Equal(expectedBuildID))
					Expect(planID).To(Equal(failureTaskPlan.ID))
					Expect(sourceName).To(Equal(worker.ArtifactName("some-failure-task")))
					Expect(workerMetadata).To(Equal(db.ContainerMetadata{
						PipelineID:   expectedPipelineID,
						PipelineName: "some-pipeline",
						JobID:        expectedJobID,
						JobName:      "some-job",
						BuildID:      expectedBuildID,
						BuildName:    "42",
						StepName:     "some-failure-task",
						Type:         db.ContainerTypeTask,
					}))

					Expect(delegate).To(Equal(fakeExecutionDelegate))

					_, _, location := fakeDelegate.ExecutionDelegateArgsForCall(0)
					Expect(location).NotTo(BeNil())
				})

				It("constructs the success hook correctly", func() {
					Expect(fakeFactory.TaskCallCount()).To(Equal(4))
					logger, teamID, buildID, planID, sourceName, workerMetadata, delegate, _, _, _, _, _, _, _, _ := fakeFactory.TaskArgsForCall(1)
					Expect(logger).NotTo(BeNil())
					Expect(teamID).To(Equal(expectedTeamID))
					Expect(buildID).To(Equal(expectedBuildID))
					Expect(planID).To(Equal(successTaskPlan.ID))
					Expect(sourceName).To(Equal(worker.ArtifactName("some-success-task")))
					Expect(workerMetadata).To(Equal(db.ContainerMetadata{
						PipelineID:   expectedPipelineID,
						PipelineName: "some-pipeline",
						JobID:        expectedJobID,
						JobName:      "some-job",
						BuildID:      expectedBuildID,
						BuildName:    "42",
						StepName:     "some-success-task",
						Type:         db.ContainerTypeTask,
					}))

					Expect(delegate).To(Equal(fakeExecutionDelegate))

					_, _, location := fakeDelegate.ExecutionDelegateArgsForCall(1)
					Expect(location).NotTo(BeNil())
				})

				It("constructs the next step correctly", func() {
					Expect(fakeFactory.TaskCallCount()).To(Equal(4))
					logger, teamID, buildID, planID, sourceName, workerMetadata, delegate, _, _, _, _, _, _, _, _ := fakeFactory.TaskArgsForCall(3)
					Expect(logger).NotTo(BeNil())
					Expect(teamID).To(Equal(expectedTeamID))
					Expect(buildID).To(Equal(expectedBuildID))
					Expect(planID).To(Equal(nextTaskPlan.ID))
					Expect(sourceName).To(Equal(worker.ArtifactName("some-next-task")))
					Expect(workerMetadata).To(Equal(db.ContainerMetadata{
						PipelineID:   expectedPipelineID,
						PipelineName: "some-pipeline",
						JobID:        expectedJobID,
						JobName:      "some-job",
						BuildID:      expectedBuildID,
						BuildName:    "42",
						StepName:     "some-next-task",
						Type:         db.ContainerTypeTask,
					}))

					Expect(delegate).To(Equal(fakeExecutionDelegate))
					_, _, location := fakeDelegate.ExecutionDelegateArgsForCall(3)
					Expect(location).NotTo(BeNil())
				})
			})
		})

		Context("when the step succeeds", func() {
			var planFactory atc.PlanFactory

			BeforeEach(func() {
				planFactory = atc.NewPlanFactory(123)
				inputStep.ResultStub = successResult(true)
			})

			It("runs the next step", func() {
				plan := planFactory.NewPlan(atc.OnSuccessPlan{
					Step: planFactory.NewPlan(atc.GetPlan{
						Name: "some-input",
					}),
					Next: planFactory.NewPlan(atc.TaskPlan{
						Name:   "some-resource",
						Config: &atc.LoadTaskConfig{},
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
							Config: &atc.LoadTaskConfig{},
						}),
					}),
					Next: planFactory.NewPlan(atc.OnSuccessPlan{
						Step: planFactory.NewPlan(atc.PutPlan{
							Name: "some-put",
						}),
						Next: planFactory.NewPlan(atc.DependentGetPlan{
							Name: "some-put",
						}),
					}),
				})

				build, err := execEngine.CreateBuild(logger, build, plan)

				Expect(err).NotTo(HaveOccurred())

				build.Resume(logger)

				Expect(inputStep.RunCallCount()).To(Equal(1))

				Expect(taskStep.RunCallCount()).To(Equal(1))

				Expect(outputStep.RunCallCount()).To(Equal(1))

				Expect(dependentStep.RunCallCount()).To(Equal(1))
			})

			Context("when the success hook fails, and has a failure hook", func() {
				BeforeEach(func() {
					taskStep.ResultStub = successResult(false)
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
									Config: &atc.LoadTaskConfig{},
								}),
								Next: planFactory.NewPlan(atc.TaskPlan{
									Name:   "some-input-success-failure",
									Config: &atc.LoadTaskConfig{},
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

					Expect(dependentStep.RunCallCount()).To(Equal(0))
				})
			})
		})

		Context("when the step fails", func() {
			var planFactory atc.PlanFactory

			BeforeEach(func() {
				planFactory = atc.NewPlanFactory(123)
				inputStep.ResultStub = successResult(false)
			})

			It("only run the failure hooks", func() {
				plan := planFactory.NewPlan(atc.OnSuccessPlan{
					Step: planFactory.NewPlan(atc.OnFailurePlan{
						Step: planFactory.NewPlan(atc.GetPlan{
							Name: "some-input",
						}),
						Next: planFactory.NewPlan(atc.TaskPlan{
							Name:   "some-resource",
							Config: &atc.LoadTaskConfig{},
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

				_, cbErr, successful, aborted := fakeDelegate.FinishArgsForCall(0)
				Expect(cbErr).NotTo(HaveOccurred())
				Expect(successful).To(Equal(exec.Success(false)))
				Expect(aborted).To(BeFalse())
			})
		})

		Context("when a step in the aggregate fails the step fails", func() {
			var planFactory atc.PlanFactory

			BeforeEach(func() {
				planFactory = atc.NewPlanFactory(123)
				inputStep.ResultStub = successResult(false)
			})

			It("only run the failure hooks", func() {
				plan := planFactory.NewPlan(atc.OnSuccessPlan{
					Step: planFactory.NewPlan(atc.AggregatePlan{
						planFactory.NewPlan(atc.TaskPlan{
							Name:   "some-resource",
							Config: &atc.LoadTaskConfig{},
						}),
						planFactory.NewPlan(atc.OnFailurePlan{
							Step: planFactory.NewPlan(atc.GetPlan{
								Name: "some-input",
							}),
							Next: planFactory.NewPlan(atc.TaskPlan{
								Name:   "some-resource",
								Config: &atc.LoadTaskConfig{},
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
