package engine_test

import (
	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/engine"
	"github.com/concourse/atc/engine/fakes"
	"github.com/concourse/atc/exec"
	execfakes "github.com/concourse/atc/exec/fakes"
	"github.com/concourse/atc/worker"
	"github.com/pivotal-golang/lager/lagertest"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Exec Engine With Hooks", func() {
	var (
		fakeFactory         *execfakes.FakeFactory
		fakeDelegateFactory *fakes.FakeBuildDelegateFactory
		fakeDB              *fakes.FakeEngineDB

		execEngine engine.Engine

		buildModel       db.Build
		expectedMetadata engine.StepMetadata
		logger           *lagertest.TestLogger

		fakeDelegate *fakes.FakeBuildDelegate
	)

	BeforeEach(func() {
		logger = lagertest.NewTestLogger("test")

		fakeFactory = new(execfakes.FakeFactory)
		fakeDelegateFactory = new(fakes.FakeBuildDelegateFactory)
		fakeDB = new(fakes.FakeEngineDB)

		execEngine = engine.NewExecEngine(fakeFactory, fakeDelegateFactory, fakeDB)

		fakeDelegate = new(fakes.FakeBuildDelegate)
		fakeDelegateFactory.DelegateReturns(fakeDelegate)

		buildModel = db.Build{
			ID:           84,
			Name:         "42",
			JobName:      "some-job",
			PipelineName: "some-pipeline",
		}

		expectedMetadata = engine.StepMetadata{
			BuildID:      84,
			BuildName:    "42",
			JobName:      "some-job",
			PipelineName: "some-pipeline",
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
				fakeDelegate          *fakes.FakeBuildDelegate
				fakeInputDelegate     *execfakes.FakeGetDelegate
				fakeExecutionDelegate *execfakes.FakeTaskDelegate
			)

			BeforeEach(func() {
				fakeDelegate = new(fakes.FakeBuildDelegate)
				fakeDelegateFactory.DelegateReturns(fakeDelegate)

				fakeInputDelegate = new(execfakes.FakeGetDelegate)
				fakeDelegate.InputDelegateReturns(fakeInputDelegate)

				fakeExecutionDelegate = new(execfakes.FakeTaskDelegate)
				fakeDelegate.ExecutionDelegateReturns(fakeExecutionDelegate)
			})

			Context("with nested aggregates in hooks", func() {
				BeforeEach(func() {
					plan := atc.Plan{
						Location: &atc.Location{},
						OnSuccess: &atc.OnSuccessPlan{
							Step: atc.Plan{
								Location: &atc.Location{},
								Get: &atc.GetPlan{
									Name: "some-input",
								},
							},
							Next: atc.Plan{
								Location: &atc.Location{},
								Aggregate: &atc.AggregatePlan{
									atc.Plan{
										Location: &atc.Location{},
										OnSuccess: &atc.OnSuccessPlan{
											Step: atc.Plan{
												Location: &atc.Location{
													Hook: "success",
												},
												Task: &atc.TaskPlan{
													Name:   "some-success-task-1",
													Config: &atc.TaskConfig{},
												},
											},
											Next: atc.Plan{
												Location: &atc.Location{
													Hook: "success",
												},
												Get: &atc.GetPlan{
													Name: "some-input",
												},
											},
										},
									},
									atc.Plan{
										Location: &atc.Location{},
										Aggregate: &atc.AggregatePlan{
											atc.Plan{
												Location: &atc.Location{},
												Task: &atc.TaskPlan{
													Name:   "some-success-task-2",
													Config: &atc.TaskConfig{},
												},
											},
										},
									},
									atc.Plan{
										Location: &atc.Location{
											Hook: "success",
										},
										Task: &atc.TaskPlan{
											Name:   "some-success-task-3",
											Config: &atc.TaskConfig{},
										},
									},
								},
							},
						},
					}

					build, err := execEngine.CreateBuild(logger, buildModel, plan)
					Expect(err).NotTo(HaveOccurred())
					build.Resume(logger)
				})

				It("constructs the steps correctly", func() {
					Expect(fakeFactory.TaskCallCount()).To(Equal(3))
					logger, sourceName, workerID, delegate, _, _, _ := fakeFactory.TaskArgsForCall(0)
					Expect(logger).NotTo(BeNil())
					Expect(sourceName).To(Equal(exec.SourceName("some-success-task-1")))
					Expect(workerID).To(Equal(worker.Identifier{
						BuildID: 84,
						Type:    db.ContainerTypeTask,
						Name:    "some-success-task-1",
					}))

					Expect(delegate).To(Equal(fakeExecutionDelegate))

					Expect(fakeFactory.GetCallCount()).To(Equal(2))
					logger, metadata, sourceName, workerID, getDelegate, _, _, _, _ := fakeFactory.GetArgsForCall(1)
					Expect(logger).NotTo(BeNil())
					Expect(metadata).To(Equal(expectedMetadata))
					Expect(sourceName).To(Equal(exec.SourceName("some-input")))
					Expect(workerID).To(Equal(worker.Identifier{
						BuildID: 84,
						Type:    db.ContainerTypeGet,
						Name:    "some-input",
					}))

					Expect(getDelegate).To(Equal(fakeInputDelegate))
					_, _, location := fakeDelegate.InputDelegateArgsForCall(1)
					Expect(location).NotTo(BeNil())

					_, _, location = fakeDelegate.ExecutionDelegateArgsForCall(0)
					Expect(location).NotTo(BeNil())

					logger, sourceName, workerID, delegate, _, _, _ = fakeFactory.TaskArgsForCall(1)
					Expect(logger).NotTo(BeNil())
					Expect(sourceName).To(Equal(exec.SourceName("some-success-task-2")))
					Expect(workerID).To(Equal(worker.Identifier{
						BuildID: 84,
						Type:    db.ContainerTypeTask,
						Name:    "some-success-task-2",
					}))

					Expect(delegate).To(Equal(fakeExecutionDelegate))

					_, _, location = fakeDelegate.ExecutionDelegateArgsForCall(1)
					Expect(location).NotTo(BeNil())

					logger, sourceName, workerID, delegate, _, _, _ = fakeFactory.TaskArgsForCall(2)
					Expect(logger).NotTo(BeNil())
					Expect(sourceName).To(Equal(exec.SourceName("some-success-task-3")))
					Expect(workerID).To(Equal(worker.Identifier{
						BuildID: 84,
						Type:    db.ContainerTypeTask,
						Name:    "some-success-task-3",
					}))

					Expect(delegate).To(Equal(fakeExecutionDelegate))

					_, _, location = fakeDelegate.ExecutionDelegateArgsForCall(2)
					Expect(location).NotTo(BeNil())
				})
			})

			Context("with all the hooks", func() {
				BeforeEach(func() {
					plan := atc.Plan{
						Location: &atc.Location{},
						OnSuccess: &atc.OnSuccessPlan{
							Step: atc.Plan{
								Ensure: &atc.EnsurePlan{
									Step: atc.Plan{
										OnSuccess: &atc.OnSuccessPlan{
											Step: atc.Plan{
												OnFailure: &atc.OnFailurePlan{
													Step: atc.Plan{
														Location: &atc.Location{},
														Get: &atc.GetPlan{
															Name: "some-input",
														},
													},
													Next: atc.Plan{
														Location: &atc.Location{
															Hook: "failure",
														},
														Task: &atc.TaskPlan{
															Name:   "some-failure-task",
															Config: &atc.TaskConfig{},
														},
													},
												},
											},
											Next: atc.Plan{
												Location: &atc.Location{
													Hook: "success",
												},
												Task: &atc.TaskPlan{
													Name:   "some-success-task",
													Config: &atc.TaskConfig{},
												},
											},
										},
									},
									Next: atc.Plan{
										Location: &atc.Location{
											Hook: "ensure",
										},
										Task: &atc.TaskPlan{
											Name:   "some-completion-task",
											Config: &atc.TaskConfig{},
										},
									},
								},
							},
							Next: atc.Plan{
								Location: &atc.Location{},
								Task: &atc.TaskPlan{
									Name:   "some-next-task",
									Config: &atc.TaskConfig{},
								},
							},
						},
					}

					build, err := execEngine.CreateBuild(logger, buildModel, plan)
					Expect(err).NotTo(HaveOccurred())
					build.Resume(logger)
				})

				It("constructs the step correctly", func() {
					Expect(fakeFactory.GetCallCount()).To(Equal(1))
					logger, metadata, sourceName, workerID, delegate, _, _, _, _ := fakeFactory.GetArgsForCall(0)
					Expect(logger).NotTo(BeNil())
					Expect(metadata).To(Equal(expectedMetadata))
					Expect(sourceName).To(Equal(exec.SourceName("some-input")))
					Expect(workerID).To(Equal(worker.Identifier{
						BuildID: 84,
						Type:    db.ContainerTypeGet,
						Name:    "some-input",
					}))

					Expect(delegate).To(Equal(fakeInputDelegate))
					_, _, location := fakeDelegate.InputDelegateArgsForCall(0)
					Expect(location).NotTo(BeNil())
				})

				It("constructs the completion hook correctly", func() {
					Expect(fakeFactory.TaskCallCount()).To(Equal(4))
					logger, sourceName, workerID, delegate, _, _, _ := fakeFactory.TaskArgsForCall(2)
					Expect(logger).NotTo(BeNil())
					Expect(sourceName).To(Equal(exec.SourceName("some-completion-task")))
					Expect(workerID).To(Equal(worker.Identifier{
						BuildID: 84,
						Type:    db.ContainerTypeTask,
						Name:    "some-completion-task",
					}))

					Expect(delegate).To(Equal(fakeExecutionDelegate))

					_, _, location := fakeDelegate.ExecutionDelegateArgsForCall(2)
					Expect(location).NotTo(BeNil())
				})

				It("constructs the failure hook correctly", func() {
					Expect(fakeFactory.TaskCallCount()).To(Equal(4))
					logger, sourceName, workerID, delegate, _, _, _ := fakeFactory.TaskArgsForCall(0)
					Expect(logger).NotTo(BeNil())
					Expect(sourceName).To(Equal(exec.SourceName("some-failure-task")))
					Expect(workerID).To(Equal(worker.Identifier{
						BuildID: 84,
						Type:    db.ContainerTypeTask,
						Name:    "some-failure-task",
					}))

					Expect(delegate).To(Equal(fakeExecutionDelegate))

					_, _, location := fakeDelegate.ExecutionDelegateArgsForCall(0)
					Expect(location).NotTo(BeNil())
				})

				It("constructs the success hook correctly", func() {
					Expect(fakeFactory.TaskCallCount()).To(Equal(4))
					logger, sourceName, workerID, delegate, _, _, _ := fakeFactory.TaskArgsForCall(1)
					Expect(logger).NotTo(BeNil())
					Expect(sourceName).To(Equal(exec.SourceName("some-success-task")))
					Expect(workerID).To(Equal(worker.Identifier{
						BuildID: 84,
						Type:    db.ContainerTypeTask,
						Name:    "some-success-task",
					}))

					Expect(delegate).To(Equal(fakeExecutionDelegate))

					_, _, location := fakeDelegate.ExecutionDelegateArgsForCall(1)
					Expect(location).NotTo(BeNil())
				})

				It("constructs the next step correctly", func() {
					Expect(fakeFactory.TaskCallCount()).To(Equal(4))
					logger, sourceName, workerID, delegate, _, _, _ := fakeFactory.TaskArgsForCall(3)
					Expect(logger).NotTo(BeNil())
					Expect(sourceName).To(Equal(exec.SourceName("some-next-task")))
					Expect(workerID).To(Equal(worker.Identifier{
						BuildID: 84,
						Type:    db.ContainerTypeTask,
						Name:    "some-next-task",
					}))

					Expect(delegate).To(Equal(fakeExecutionDelegate))
					_, _, location := fakeDelegate.ExecutionDelegateArgsForCall(3)
					Expect(location).NotTo(BeNil())

				})
			})
		})

		Context("when the step succeeds", func() {
			BeforeEach(func() {
				inputStep.ResultStub = successResult(true)
			})

			It("runs the next step", func() {
				plan := atc.Plan{
					Location: &atc.Location{},
					OnSuccess: &atc.OnSuccessPlan{
						Step: atc.Plan{
							Location: &atc.Location{},
							Get: &atc.GetPlan{
								Name: "some-input",
							},
						},
						Next: atc.Plan{
							Location: &atc.Location{},
							Task: &atc.TaskPlan{
								Name:   "some-resource",
								Config: &atc.TaskConfig{},
							},
						},
					},
				}

				build, err := execEngine.CreateBuild(logger, buildModel, plan)

				Expect(err).NotTo(HaveOccurred())

				build.Resume(logger)

				Expect(inputStep.RunCallCount()).To(Equal(1))
				Expect(inputStep.ReleaseCallCount()).To(Equal(1))

				Expect(taskStep.RunCallCount()).To(Equal(1))
				Expect(taskStep.ReleaseCallCount()).To(Equal(1))
			})

			It("runs the success hooks, and completion hooks", func() {
				plan := atc.Plan{
					Location: &atc.Location{},
					Ensure: &atc.EnsurePlan{
						Step: atc.Plan{
							OnSuccess: &atc.OnSuccessPlan{
								Step: atc.Plan{
									Location: &atc.Location{},
									Get: &atc.GetPlan{
										Name: "some-input",
									},
								},
								Next: atc.Plan{
									Location: &atc.Location{
										Hook: "success",
									},
									Task: &atc.TaskPlan{
										Name:   "some-resource",
										Config: &atc.TaskConfig{},
									},
								},
							},
						},
						Next: atc.Plan{
							OnSuccess: &atc.OnSuccessPlan{
								Step: atc.Plan{
									Location: &atc.Location{
										Hook: "ensure",
									},
									Put: &atc.PutPlan{
										Name: "some-put",
									},
								},
								Next: atc.Plan{
									Location: &atc.Location{},
									DependentGet: &atc.DependentGetPlan{
										Name: "some-put",
									},
								},
							},
						},
					},
				}

				build, err := execEngine.CreateBuild(logger, buildModel, plan)

				Expect(err).NotTo(HaveOccurred())

				build.Resume(logger)

				Expect(inputStep.RunCallCount()).To(Equal(1))
				Expect(inputStep.ReleaseCallCount()).To(Equal(1))

				Expect(taskStep.RunCallCount()).To(Equal(1))
				Expect(taskStep.ReleaseCallCount()).To(Equal(1))

				Expect(outputStep.RunCallCount()).To(Equal(1))
				Expect(outputStep.ReleaseCallCount()).To(Equal(1))

				Expect(dependentStep.RunCallCount()).To(Equal(1))
				Expect(dependentStep.ReleaseCallCount()).To(Equal(1))
			})

			Context("when the success hook fails, and has a failure hook", func() {
				BeforeEach(func() {
					taskStep.ResultStub = successResult(false)
				})

				It("does not run the next step", func() {
					plan := atc.Plan{
						Location: &atc.Location{},
						OnSuccess: &atc.OnSuccessPlan{
							Step: atc.Plan{
								OnSuccess: &atc.OnSuccessPlan{
									Step: atc.Plan{
										Location: &atc.Location{},
										Get: &atc.GetPlan{
											Name: "some-input",
										},
									},
									Next: atc.Plan{
										Location: &atc.Location{
											Hook: "success",
										},
										OnFailure: &atc.OnFailurePlan{
											Step: atc.Plan{
												Location: &atc.Location{},
												Task: &atc.TaskPlan{
													Name:   "some-resource",
													Config: &atc.TaskConfig{},
												},
											},
											Next: atc.Plan{
												Location: &atc.Location{
													Hook: "failure",
												},
												Task: &atc.TaskPlan{
													Name:   "some-input-success-failure",
													Config: &atc.TaskConfig{},
												},
											},
										},
									},
								},
							},
							Next: atc.Plan{
								Location: &atc.Location{},
							},
						},
					}

					build, err := execEngine.CreateBuild(logger, buildModel, plan)

					Expect(err).NotTo(HaveOccurred())

					build.Resume(logger)

					Expect(inputStep.RunCallCount()).To(Equal(1))
					Expect(inputStep.ReleaseCallCount()).To(Equal(1))

					Expect(taskStep.RunCallCount()).To(Equal(2))
					Expect(inputStep.ReleaseCallCount()).To(Equal(1))

					Expect(outputStep.RunCallCount()).To(Equal(0))
					Expect(outputStep.ReleaseCallCount()).To(Equal(0))

					Expect(dependentStep.RunCallCount()).To(Equal(0))
					Expect(dependentStep.ReleaseCallCount()).To(Equal(0))
				})
			})
		})

		Context("when the step fails", func() {
			BeforeEach(func() {
				inputStep.ResultStub = successResult(false)
			})

			It("only run the failure hooks", func() {
				plan := atc.Plan{
					Location: &atc.Location{},
					OnSuccess: &atc.OnSuccessPlan{
						Step: atc.Plan{
							OnFailure: &atc.OnFailurePlan{
								Step: atc.Plan{
									Location: &atc.Location{},
									Get: &atc.GetPlan{
										Name: "some-input",
									},
								},
								Next: atc.Plan{
									Location: &atc.Location{
										Hook: "failure",
									},
									Task: &atc.TaskPlan{
										Name:   "some-resource",
										Config: &atc.TaskConfig{},
									},
								},
							},
						},
						Next: atc.Plan{
							Location: &atc.Location{
								Hook: "success",
							},
						},
					},
				}

				build, err := execEngine.CreateBuild(logger, buildModel, plan)

				Expect(err).NotTo(HaveOccurred())

				build.Resume(logger)

				Expect(inputStep.RunCallCount()).To(Equal(1))
				Expect(inputStep.ReleaseCallCount()).To(Equal(1))

				Expect(taskStep.RunCallCount()).To(Equal(1))
				Expect(inputStep.ReleaseCallCount()).To(Equal(1))

				Expect(outputStep.RunCallCount()).To(Equal(0))
				Expect(outputStep.ReleaseCallCount()).To(Equal(0))

				_, cbErr, successful, aborted := fakeDelegate.FinishArgsForCall(0)
				Expect(cbErr).NotTo(HaveOccurred())
				Expect(successful).To(Equal(exec.Success(false)))
				Expect(aborted).To(BeFalse())
			})
		})

		Context("when a step in the aggregate fails the step fails", func() {
			BeforeEach(func() {
				inputStep.ResultStub = successResult(false)
			})

			It("only run the failure hooks", func() {
				plan := atc.Plan{
					Location: &atc.Location{},
					OnSuccess: &atc.OnSuccessPlan{
						Step: atc.Plan{
							Location: &atc.Location{},
							Aggregate: &atc.AggregatePlan{
								atc.Plan{
									Location: &atc.Location{},
									Task: &atc.TaskPlan{
										Name:   "some-resource",
										Config: &atc.TaskConfig{},
									},
								},
								atc.Plan{
									Location: &atc.Location{},
									OnFailure: &atc.OnFailurePlan{
										Step: atc.Plan{
											Location: &atc.Location{},
											Get: &atc.GetPlan{
												Name: "some-input",
											},
										},
										Next: atc.Plan{
											Location: &atc.Location{
												Hook: "failure",
											},
											Task: &atc.TaskPlan{
												Name:   "some-resource",
												Config: &atc.TaskConfig{},
											},
										},
									},
								},
							},
						},
						Next: atc.Plan{
							Location: &atc.Location{},
						},
					},
				}

				build, err := execEngine.CreateBuild(logger, buildModel, plan)

				Expect(err).NotTo(HaveOccurred())

				build.Resume(logger)

				Expect(inputStep.RunCallCount()).To(Equal(1))

				Expect(taskStep.RunCallCount()).To(Equal(2))

				Expect(outputStep.RunCallCount()).To(Equal(0))
			})
		})
	})
})
