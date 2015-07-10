package engine_test

import (
	"os"

	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/engine"
	"github.com/concourse/atc/engine/fakes"
	"github.com/concourse/atc/event"
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

		buildModel db.Build
		logger     *lagertest.TestLogger

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

		buildModel = db.Build{ID: 84}
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

			assertNotReleased := func(signals <-chan os.Signal, ready chan<- struct{}) error {
				defer GinkgoRecover()
				Consistently(inputStep.ReleaseCallCount).Should(BeZero())
				Consistently(taskStep.ReleaseCallCount).Should(BeZero())
				Consistently(outputStep.ReleaseCallCount).Should(BeZero())
				return nil
			}

			taskStep.RunStub = assertNotReleased
			inputStep.RunStub = assertNotReleased
			outputStep.RunStub = assertNotReleased
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
						HookedCompose: &atc.HookedComposePlan{
							Step: atc.Plan{
								Get: &atc.GetPlan{
									Name: "some-input",
								},
							},
							OnSuccess: atc.Plan{
								Aggregate: &atc.AggregatePlan{
									atc.Plan{
										HookedCompose: &atc.HookedComposePlan{
											Step: atc.Plan{
												Task: &atc.TaskPlan{
													Name:   "some-success-task-1",
													Config: &atc.TaskConfig{},
												},
											},
											OnSuccess: atc.Plan{
												Get: &atc.GetPlan{
													Name: "some-input",
												},
											},
										},
									},
									atc.Plan{
										Aggregate: &atc.AggregatePlan{
											atc.Plan{
												Task: &atc.TaskPlan{
													Name:   "some-success-task-2",
													Config: &atc.TaskConfig{},
												},
											},
										},
									},
									atc.Plan{
										Task: &atc.TaskPlan{
											Name:   "some-success-task-3",
											Config: &atc.TaskConfig{},
										},
									},
								},
							},
						},
					}

					build, err := execEngine.CreateBuild(buildModel, plan)
					Ω(err).ShouldNot(HaveOccurred())
					build.Resume(logger)
				})

				It("constructs the steps correctly", func() {
					Ω(fakeFactory.TaskCallCount()).Should(Equal(3))
					sourceName, workerID, delegate, _, _, _ := fakeFactory.TaskArgsForCall(0)
					Ω(sourceName).Should(Equal(exec.SourceName("some-success-task-1")))
					Ω(workerID).Should(Equal(worker.Identifier{
						BuildID:      84,
						Type:         worker.ContainerTypeTask,
						Name:         "some-success-task-1",
						StepLocation: 3,
					}))
					Ω(delegate).Should(Equal(fakeExecutionDelegate))

					Ω(fakeFactory.GetCallCount()).Should(Equal(2))
					sourceName, workerID, getDelegate, _, _, _, _ := fakeFactory.GetArgsForCall(1)
					Ω(sourceName).Should(Equal(exec.SourceName("some-input")))
					Ω(workerID).Should(Equal(worker.Identifier{
						BuildID:      84,
						Type:         worker.ContainerTypeGet,
						Name:         "some-input",
						StepLocation: 4,
					}))

					Ω(getDelegate).Should(Equal(fakeInputDelegate))
					_, _, location, hook := fakeDelegate.InputDelegateArgsForCall(1)
					Ω(location).Should(Equal(event.OriginLocation{
						ParentID:      3,
						ID:            4,
						ParallelGroup: 0,
					}))
					Ω(hook).Should(Equal("success"))

					_, _, location, hook = fakeDelegate.ExecutionDelegateArgsForCall(0)
					Ω(location).Should(Equal(event.OriginLocation{
						ParentID:      1,
						ID:            3,
						ParallelGroup: 2,
					}))
					Ω(hook).Should(Equal("success"))

					sourceName, workerID, delegate, _, _, _ = fakeFactory.TaskArgsForCall(1)
					Ω(sourceName).Should(Equal(exec.SourceName("some-success-task-2")))
					Ω(workerID).Should(Equal(worker.Identifier{
						BuildID:      84,
						Type:         worker.ContainerTypeTask,
						Name:         "some-success-task-2",
						StepLocation: 6,
					}))
					Ω(delegate).Should(Equal(fakeExecutionDelegate))

					_, _, location, hook = fakeDelegate.ExecutionDelegateArgsForCall(1)
					Ω(location).Should(Equal(event.OriginLocation{
						ParentID:      2,
						ID:            6,
						ParallelGroup: 5,
					}))
					Ω(hook).Should(Equal(""))

					sourceName, workerID, delegate, _, _, _ = fakeFactory.TaskArgsForCall(2)
					Ω(sourceName).Should(Equal(exec.SourceName("some-success-task-3")))
					Ω(workerID).Should(Equal(worker.Identifier{
						BuildID:      84,
						Type:         worker.ContainerTypeTask,
						Name:         "some-success-task-3",
						StepLocation: 7,
					}))
					Ω(delegate).Should(Equal(fakeExecutionDelegate))

					_, _, location, hook = fakeDelegate.ExecutionDelegateArgsForCall(2)
					Ω(location).Should(Equal(event.OriginLocation{
						ParentID:      1,
						ID:            7,
						ParallelGroup: 2,
					}))
					Ω(hook).Should(Equal("success"))
				})
			})

			Context("with all the hooks", func() {
				BeforeEach(func() {
					plan := atc.Plan{
						HookedCompose: &atc.HookedComposePlan{
							Step: atc.Plan{
								Get: &atc.GetPlan{
									Name: "some-input",
								},
							},
							OnSuccess: atc.Plan{
								Task: &atc.TaskPlan{
									Name:   "some-success-task",
									Config: &atc.TaskConfig{},
								},
							},
							OnFailure: atc.Plan{
								Task: &atc.TaskPlan{
									Name:   "some-failure-task",
									Config: &atc.TaskConfig{},
								},
							},
							OnCompletion: atc.Plan{
								Task: &atc.TaskPlan{
									Name:   "some-completion-task",
									Config: &atc.TaskConfig{},
								},
							},
							Next: atc.Plan{
								Task: &atc.TaskPlan{
									Name:   "some-next-task",
									Config: &atc.TaskConfig{},
								},
							},
						},
					}

					build, err := execEngine.CreateBuild(buildModel, plan)
					Ω(err).ShouldNot(HaveOccurred())
					build.Resume(logger)
				})

				It("constructs the step correctly", func() {
					Ω(fakeFactory.GetCallCount()).Should(Equal(1))
					sourceName, workerID, delegate, _, _, _, _ := fakeFactory.GetArgsForCall(0)
					Ω(sourceName).Should(Equal(exec.SourceName("some-input")))
					Ω(workerID).Should(Equal(worker.Identifier{
						BuildID:      84,
						Type:         worker.ContainerTypeGet,
						Name:         "some-input",
						StepLocation: 1,
					}))

					Ω(delegate).Should(Equal(fakeInputDelegate))
					_, _, location, hook := fakeDelegate.InputDelegateArgsForCall(0)
					Ω(location).Should(Equal(event.OriginLocation{
						ParentID:      0,
						ID:            1,
						ParallelGroup: 0,
					}))
					Ω(hook).Should(Equal(""))
				})

				It("constructs the completion hook correctly", func() {
					Ω(fakeFactory.TaskCallCount()).Should(Equal(4))
					sourceName, workerID, delegate, _, _, _ := fakeFactory.TaskArgsForCall(2)
					Ω(sourceName).Should(Equal(exec.SourceName("some-completion-task")))
					Ω(workerID).Should(Equal(worker.Identifier{
						BuildID:      84,
						Type:         worker.ContainerTypeTask,
						Name:         "some-completion-task",
						StepLocation: 4,
					}))
					Ω(delegate).Should(Equal(fakeExecutionDelegate))

					_, _, location, hook := fakeDelegate.ExecutionDelegateArgsForCall(2)
					Ω(location).Should(Equal(event.OriginLocation{
						ParentID:      1,
						ID:            4,
						ParallelGroup: 0,
					}))
					Ω(hook).Should(Equal("ensure"))
				})

				It("constructs the failure hook correctly", func() {
					Ω(fakeFactory.TaskCallCount()).Should(Equal(4))
					sourceName, workerID, delegate, _, _, _ := fakeFactory.TaskArgsForCall(0)
					Ω(sourceName).Should(Equal(exec.SourceName("some-failure-task")))
					Ω(workerID).Should(Equal(worker.Identifier{
						BuildID:      84,
						Type:         worker.ContainerTypeTask,
						Name:         "some-failure-task",
						StepLocation: 2,
					}))
					Ω(delegate).Should(Equal(fakeExecutionDelegate))

					_, _, location, hook := fakeDelegate.ExecutionDelegateArgsForCall(0)
					Ω(location).Should(Equal(event.OriginLocation{
						ParentID:      1,
						ID:            2,
						ParallelGroup: 0,
					}))
					Ω(hook).Should(Equal("failure"))
				})

				It("constructs the success hook correctly", func() {
					Ω(fakeFactory.TaskCallCount()).Should(Equal(4))
					sourceName, workerID, delegate, _, _, _ := fakeFactory.TaskArgsForCall(1)
					Ω(sourceName).Should(Equal(exec.SourceName("some-success-task")))
					Ω(workerID).Should(Equal(worker.Identifier{
						BuildID:      84,
						Type:         worker.ContainerTypeTask,
						Name:         "some-success-task",
						StepLocation: 3,
					}))
					Ω(delegate).Should(Equal(fakeExecutionDelegate))

					_, _, location, hook := fakeDelegate.ExecutionDelegateArgsForCall(1)
					Ω(location).Should(Equal(event.OriginLocation{
						ParentID:      1,
						ID:            3,
						ParallelGroup: 0,
					}))
					Ω(hook).Should(Equal("success"))
				})

				It("constructs the next step correctly", func() {
					Ω(fakeFactory.TaskCallCount()).Should(Equal(4))
					sourceName, workerID, delegate, _, _, _ := fakeFactory.TaskArgsForCall(3)
					Ω(sourceName).Should(Equal(exec.SourceName("some-next-task")))
					Ω(workerID).Should(Equal(worker.Identifier{
						BuildID:      84,
						Type:         worker.ContainerTypeTask,
						Name:         "some-next-task",
						StepLocation: 5,
					}))
					Ω(delegate).Should(Equal(fakeExecutionDelegate))
					_, _, location, hook := fakeDelegate.ExecutionDelegateArgsForCall(3)
					Ω(location).Should(Equal(event.OriginLocation{
						ParentID:      0,
						ID:            5,
						ParallelGroup: 0,
					}))

					Ω(hook).Should(Equal(""))
				})
			})
		})

		Context("when the step succeeds", func() {
			BeforeEach(func() {
				inputStep.ResultStub = successResult(true)
			})

			It("runs the next step", func() {
				plan := atc.Plan{
					HookedCompose: &atc.HookedComposePlan{
						Step: atc.Plan{
							Get: &atc.GetPlan{
								Name: "some-input",
							},
						},
						Next: atc.Plan{
							Task: &atc.TaskPlan{
								Name:   "some-resource",
								Config: &atc.TaskConfig{},
							},
						},
					},
				}

				build, err := execEngine.CreateBuild(buildModel, plan)

				Ω(err).ShouldNot(HaveOccurred())

				build.Resume(logger)

				Ω(inputStep.RunCallCount()).Should(Equal(1))
				// The hooked compose will try and run the next step regardless.
				// If the step is nil, we will use an identity step, which defaults to
				// returning whatever the previous step was from using.
				// For this reason, the input step gets returned as the next step of type
				// identity step, which returns nil when ran.
				Ω(inputStep.ReleaseCallCount()).Should(Equal(3))

				Ω(taskStep.RunCallCount()).Should(Equal(1))
				Ω(taskStep.ReleaseCallCount()).Should(Equal(1))
			})

			It("runs the success hooks, and completion hooks", func() {
				plan := atc.Plan{
					HookedCompose: &atc.HookedComposePlan{
						Step: atc.Plan{
							Get: &atc.GetPlan{
								Name: "some-input",
							},
						},
						OnSuccess: atc.Plan{
							Task: &atc.TaskPlan{
								Name:   "some-resource",
								Config: &atc.TaskConfig{},
							},
						},
						OnCompletion: atc.Plan{
							PutGet: &atc.PutGetPlan{
								Head: atc.Plan{
									Put: &atc.PutPlan{
										Name: "some-put",
									},
								},
							},
						},
					},
				}

				build, err := execEngine.CreateBuild(buildModel, plan)

				Ω(err).ShouldNot(HaveOccurred())

				build.Resume(logger)

				Ω(inputStep.RunCallCount()).Should(Equal(1))
				Ω(inputStep.ReleaseCallCount()).Should(Equal(2))

				Ω(taskStep.RunCallCount()).Should(Equal(1))
				Ω(taskStep.ReleaseCallCount()).Should(Equal(1))

				Ω(outputStep.RunCallCount()).Should(Equal(1))
				Ω(outputStep.ReleaseCallCount()).Should(Equal(3))

				Ω(dependentStep.RunCallCount()).Should(Equal(1))
				Ω(dependentStep.ReleaseCallCount()).Should(Equal(1))
			})

			Context("when the success hook fails, and has a failure hook", func() {
				BeforeEach(func() {
					taskStep.ResultStub = successResult(false)
				})

				It("does not run the next step", func() {
					plan := atc.Plan{
						HookedCompose: &atc.HookedComposePlan{
							Step: atc.Plan{
								Get: &atc.GetPlan{
									Name: "some-input",
								},
							},
							OnSuccess: atc.Plan{
								HookedCompose: &atc.HookedComposePlan{
									Step: atc.Plan{
										Task: &atc.TaskPlan{
											Name:   "some-resource",
											Config: &atc.TaskConfig{},
										},
									},
									OnFailure: atc.Plan{
										Task: &atc.TaskPlan{
											Name:   "some-input-success-failure",
											Config: &atc.TaskConfig{},
										},
									},
								},
							},
							Next: atc.Plan{
								PutGet: &atc.PutGetPlan{
									Head: atc.Plan{
										Put: &atc.PutPlan{
											Name: "some-put",
										},
									},
								},
							},
						},
					}

					build, err := execEngine.CreateBuild(buildModel, plan)

					Ω(err).ShouldNot(HaveOccurred())

					build.Resume(logger)

					Ω(inputStep.RunCallCount()).Should(Equal(1))
					Ω(inputStep.ReleaseCallCount()).Should(Equal(2))

					Ω(taskStep.RunCallCount()).Should(Equal(2))
					Ω(taskStep.ReleaseCallCount()).Should(Equal(3))

					Ω(outputStep.RunCallCount()).Should(Equal(0))
					Ω(outputStep.ReleaseCallCount()).Should(Equal(0))

					Ω(dependentStep.RunCallCount()).Should(Equal(0))
					Ω(dependentStep.ReleaseCallCount()).Should(Equal(0))
				})
			})
		})

		Context("when the step fails", func() {
			BeforeEach(func() {
				inputStep.ResultStub = successResult(false)
			})

			It("only run the failure hooks", func() {
				plan := atc.Plan{
					HookedCompose: &atc.HookedComposePlan{
						Step: atc.Plan{
							Get: &atc.GetPlan{
								Name: "some-input",
							},
						},
						OnFailure: atc.Plan{
							Task: &atc.TaskPlan{
								Name:   "some-resource",
								Config: &atc.TaskConfig{},
							},
						},
						OnSuccess: atc.Plan{
							PutGet: &atc.PutGetPlan{
								Head: atc.Plan{
									Put: &atc.PutPlan{
										Name: "some-put",
									},
								},
							},
						},
					},
				}

				build, err := execEngine.CreateBuild(buildModel, plan)

				Ω(err).ShouldNot(HaveOccurred())

				build.Resume(logger)

				Ω(inputStep.RunCallCount()).Should(Equal(1))
				Ω(inputStep.ReleaseCallCount()).Should(Equal(2))

				Ω(taskStep.RunCallCount()).Should(Equal(1))
				Ω(taskStep.ReleaseCallCount()).Should(Equal(1))

				Ω(outputStep.RunCallCount()).Should(Equal(0))
				Ω(outputStep.ReleaseCallCount()).Should(Equal(0))
			})
		})
	})
})
