package engine_test

import (
	"errors"
	"os"

	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/engine"
	"github.com/concourse/atc/engine/fakes"
	"github.com/concourse/atc/event"
	"github.com/concourse/atc/exec"
	execfakes "github.com/concourse/atc/exec/fakes"
	"github.com/concourse/atc/worker"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pivotal-golang/lager/lagertest"
)

var _ = Describe("ExecEngine", func() {
	var (
		fakeFactory         *execfakes.FakeFactory
		fakeDelegateFactory *fakes.FakeBuildDelegateFactory
		fakeDB              *fakes.FakeEngineDB

		execEngine engine.Engine
	)

	BeforeEach(func() {
		fakeFactory = new(execfakes.FakeFactory)
		fakeDelegateFactory = new(fakes.FakeBuildDelegateFactory)
		fakeDB = new(fakes.FakeEngineDB)

		execEngine = engine.NewExecEngine(fakeFactory, fakeDelegateFactory, fakeDB)
	})

	Describe("Resume", func() {
		var (
			fakeDelegate          *fakes.FakeBuildDelegate
			fakeInputDelegate     *execfakes.FakeGetDelegate
			fakeExecutionDelegate *execfakes.FakeTaskDelegate
			fakeOutputDelegate    *execfakes.FakePutDelegate

			buildModel db.Build

			inputPlan  *atc.GetPlan
			outputPlan *atc.ConditionalPlan
			privileged bool
			taskConfig *atc.TaskConfig

			taskConfigPath string

			build engine.Build

			logger *lagertest.TestLogger

			inputStepFactory *execfakes.FakeStepFactory
			inputStep        *execfakes.FakeStep

			taskStepFactory *execfakes.FakeStepFactory
			taskStep        *execfakes.FakeStep

			outputStepFactory *execfakes.FakeStepFactory
			outputStep        *execfakes.FakeStep

			dependentStepFactory *execfakes.FakeStepFactory
			dependentStep        *execfakes.FakeStep
		)

		BeforeEach(func() {
			logger = lagertest.NewTestLogger("test")

			buildModel = db.Build{ID: 42}

			taskConfig = &atc.TaskConfig{
				Image:  "some-image",
				Tags:   []string{"some", "task", "tags"},
				Params: map[string]string{"PARAM": "value"},
				Run: atc.TaskRunConfig{
					Path: "some-path",
					Args: []string{"some", "args"},
				},
				Inputs: []atc.TaskInputConfig{
					{Name: "some-input"},
				},
			}

			taskConfigPath = "some-input/build.yml"

			inputPlan = &atc.GetPlan{
				Name:     "some-input",
				Resource: "some-input-resource",
				Type:     "some-type",
				Tags:     []string{"some", "get", "tags"},
				Version:  atc.Version{"some": "version"},
				Source:   atc.Source{"some": "source"},
				Params:   atc.Params{"some": "params"},
			}

			outputPlan = &atc.ConditionalPlan{
				Conditions: atc.Conditions{atc.ConditionSuccess},
				Plan: atc.Plan{
					PutGet: &atc.PutGetPlan{
						Head: atc.Plan{
							Put: &atc.PutPlan{
								Name:      "some-put",
								Resource:  "some-output-resource",
								Tags:      []string{"some", "putget", "tags"},
								Type:      "some-type",
								Source:    atc.Source{"some": "source"},
								Params:    atc.Params{"some": "params"},
								GetParams: atc.Params{"another": "params"},
							},
						},
					},
				},
			}

			privileged = false

			fakeDelegate = new(fakes.FakeBuildDelegate)
			fakeDelegateFactory.DelegateReturns(fakeDelegate)

			fakeInputDelegate = new(execfakes.FakeGetDelegate)
			fakeDelegate.InputDelegateReturns(fakeInputDelegate)

			fakeExecutionDelegate = new(execfakes.FakeTaskDelegate)
			fakeDelegate.ExecutionDelegateReturns(fakeExecutionDelegate)

			fakeOutputDelegate = new(execfakes.FakePutDelegate)
			fakeDelegate.OutputDelegateReturns(fakeOutputDelegate)

			inputStepFactory = new(execfakes.FakeStepFactory)
			inputStep = new(execfakes.FakeStep)
			inputStepFactory.UsingReturns(inputStep)
			fakeFactory.GetReturns(inputStepFactory)

			taskStepFactory = new(execfakes.FakeStepFactory)
			taskStep = new(execfakes.FakeStep)
			taskStep.ResultStub = successResult(true)
			taskStepFactory.UsingReturns(taskStep)
			fakeFactory.TaskReturns(taskStepFactory)

			outputStepFactory = new(execfakes.FakeStepFactory)
			outputStep = new(execfakes.FakeStep)
			outputStepFactory.UsingReturns(outputStep)
			fakeFactory.PutReturns(outputStepFactory)

			dependentStepFactory = new(execfakes.FakeStepFactory)
			dependentStep = new(execfakes.FakeStep)
			dependentStepFactory.UsingReturns(dependentStep)
			fakeFactory.DependentGetReturns(dependentStepFactory)
		})

		JustBeforeEach(func() {
			var err error
			build, err = execEngine.CreateBuild(buildModel, atc.Plan{
				Compose: &atc.ComposePlan{
					A: atc.Plan{
						Aggregate: &atc.AggregatePlan{
							atc.Plan{
								Get: inputPlan,
							},
						},
					},
					B: atc.Plan{
						Compose: &atc.ComposePlan{
							A: atc.Plan{
								Task: &atc.TaskPlan{
									Name: "some-task",

									Privileged: privileged,

									Config:     taskConfig,
									ConfigPath: taskConfigPath,
								},
							},
							B: atc.Plan{
								Aggregate: &atc.AggregatePlan{
									atc.Plan{
										Conditional: outputPlan,
									},
								},
							},
						},
					},
				},
			})
			Ω(err).ShouldNot(HaveOccurred())

			build.Resume(logger)
		})

		Describe("with a putget in an aggregate", func() {
			BeforeEach(func() {
				outputPlan = &atc.ConditionalPlan{
					Conditions: atc.Conditions{atc.ConditionSuccess},
					Plan: atc.Plan{
						Aggregate: &atc.AggregatePlan{
							atc.Plan{
								Conditional: &atc.ConditionalPlan{
									Conditions: []atc.Condition{atc.ConditionSuccess},
									Plan: atc.Plan{
										PutGet: &atc.PutGetPlan{
											Head: atc.Plan{
												Put: &atc.PutPlan{
													Name:      "some-put",
													Resource:  "some-output-resource",
													Type:      "some-type",
													Source:    atc.Source{"some": "source"},
													Params:    atc.Params{"some": "params"},
													GetParams: atc.Params{"another": "params"},
												},
											},
										},
									},
								},
							},
							atc.Plan{
								Conditional: &atc.ConditionalPlan{
									Conditions: []atc.Condition{atc.ConditionSuccess},
									Plan: atc.Plan{
										PutGet: &atc.PutGetPlan{
											Head: atc.Plan{
												Put: &atc.PutPlan{
													Name:      "some-put-2",
													Resource:  "some-output-resource-2",
													Type:      "some-type-2",
													Source:    atc.Source{"some": "source-2"},
													Params:    atc.Params{"some": "params-2"},
													GetParams: atc.Params{"another": "params-2"},
												},
											},
										},
									},
								},
							},
						},
					},
				}
			})

			Context("constructing outputs", func() {
				It("constructs the put correctly", func() {
					Ω(fakeFactory.PutCallCount()).Should(Equal(2))

					workerID, delegate, resourceConfig, tags, params := fakeFactory.PutArgsForCall(0)
					Ω(workerID).Should(Equal(worker.Identifier{
						BuildID:      42,
						Type:         worker.ContainerTypePut,
						Name:         "some-output-resource",
						StepLocation: []uint{2, 0, 0},
					}))
					Ω(tags).Should(BeEmpty())
					Ω(delegate).Should(Equal(fakeOutputDelegate))
					Ω(resourceConfig.Name).Should(Equal("some-output-resource"))
					Ω(resourceConfig.Type).Should(Equal("some-type"))
					Ω(resourceConfig.Source).Should(Equal(atc.Source{"some": "source"}))
					Ω(params).Should(Equal(atc.Params{"some": "params"}))

					workerID, delegate, resourceConfig, tags, params = fakeFactory.PutArgsForCall(1)
					Ω(workerID).Should(Equal(worker.Identifier{
						BuildID:      42,
						Type:         worker.ContainerTypePut,
						Name:         "some-output-resource-2",
						StepLocation: []uint{2, 0, 2},
					}))
					Ω(tags).Should(BeEmpty())
					Ω(delegate).Should(Equal(fakeOutputDelegate))
					Ω(resourceConfig.Name).Should(Equal("some-output-resource-2"))
					Ω(resourceConfig.Type).Should(Equal("some-type-2"))
					Ω(resourceConfig.Source).Should(Equal(atc.Source{"some": "source-2"}))
					Ω(params).Should(Equal(atc.Params{"some": "params-2"}))
				})

				It("constructs the dependent get correctly", func() {
					Ω(fakeFactory.DependentGetCallCount()).Should(Equal(2))

					sourceName, workerID, delegate, resourceConfig, tags, params := fakeFactory.DependentGetArgsForCall(0)
					Ω(workerID).Should(Equal(worker.Identifier{
						BuildID:      42,
						Type:         worker.ContainerTypeGet,
						Name:         "some-put",
						StepLocation: []uint{2, 0, 1},
					}))

					Ω(tags).Should(BeEmpty())
					Ω(delegate).Should(Equal(fakeInputDelegate))
					_, plan, location, substep := fakeDelegate.InputDelegateArgsForCall(1)
					Ω(plan).Should(Equal((*outputPlan.Plan.Aggregate)[0].Conditional.Plan.PutGet.Head.Put.GetPlan()))
					Ω(location).Should(Equal(event.OriginLocation{2, 0, 1}))
					Ω(substep).Should(BeTrue())

					Ω(sourceName).Should(Equal(exec.SourceName("some-put")))
					Ω(resourceConfig.Name).Should(Equal("some-output-resource"))
					Ω(resourceConfig.Type).Should(Equal("some-type"))
					Ω(resourceConfig.Source).Should(Equal(atc.Source{"some": "source"}))
					Ω(params).Should(Equal(atc.Params{"another": "params"}))

					sourceName, workerID, delegate, resourceConfig, tags, params = fakeFactory.DependentGetArgsForCall(1)
					Ω(workerID).Should(Equal(worker.Identifier{
						BuildID:      42,
						Type:         worker.ContainerTypeGet,
						Name:         "some-put-2",
						StepLocation: []uint{2, 0, 3},
					}))

					Ω(tags).Should(BeEmpty())
					Ω(delegate).Should(Equal(fakeInputDelegate))
					_, plan, location, substep = fakeDelegate.InputDelegateArgsForCall(2)
					Ω(plan).Should(Equal((*outputPlan.Plan.Aggregate)[1].Conditional.Plan.PutGet.Head.Put.GetPlan()))
					Ω(location).Should(Equal(event.OriginLocation{2, 0, 3}))
					Ω(substep).Should(BeTrue())

					Ω(sourceName).Should(Equal(exec.SourceName("some-put-2")))
					Ω(resourceConfig.Name).Should(Equal("some-output-resource-2"))
					Ω(resourceConfig.Type).Should(Equal("some-type-2"))
					Ω(resourceConfig.Source).Should(Equal(atc.Source{"some": "source-2"}))
					Ω(params).Should(Equal(atc.Params{"another": "params-2"}))

				})
			})
		})

		It("constructs inputs correctly", func() {
			Ω(fakeFactory.GetCallCount()).Should(Equal(1))

			sourceName, workerID, delegate, resourceConfig, params, tags, version := fakeFactory.GetArgsForCall(0)
			Ω(sourceName).Should(Equal(exec.SourceName("some-input")))
			Ω(workerID).Should(Equal(worker.Identifier{
				BuildID:      42,
				Type:         worker.ContainerTypeGet,
				Name:         "some-input",
				StepLocation: []uint{0, 0},
			}))
			Ω(tags).Should(ConsistOf("some", "get", "tags"))

			Ω(delegate).Should(Equal(fakeInputDelegate))
			_, plan, location, substep := fakeDelegate.InputDelegateArgsForCall(0)
			Ω(plan).Should(Equal(*inputPlan))
			Ω(location).Should(Equal(event.OriginLocation{0, 0}))
			Ω(substep).Should(BeFalse())

			Ω(resourceConfig.Name).Should(Equal("some-input-resource"))
			Ω(resourceConfig.Type).Should(Equal("some-type"))
			Ω(resourceConfig.Source).Should(Equal(atc.Source{"some": "source"}))
			Ω(params).Should(Equal(atc.Params{"some": "params"}))
			Ω(version).Should(Equal(atc.Version{"some": "version"}))
		})

		It("constructs tasks correctly", func() {
			Ω(fakeFactory.TaskCallCount()).Should(Equal(1))

			sourceName, workerID, delegate, privileged, tags, configSource := fakeFactory.TaskArgsForCall(0)
			Ω(sourceName).Should(Equal(exec.SourceName("some-task")))
			Ω(workerID).Should(Equal(worker.Identifier{
				BuildID:      42,
				Type:         worker.ContainerTypeTask,
				Name:         "some-task",
				StepLocation: []uint{1},
			}))
			Ω(delegate).Should(Equal(fakeExecutionDelegate))
			Ω(privileged).Should(Equal(exec.Privileged(false)))
			Ω(tags).Should(BeEmpty())
			Ω(configSource).ShouldNot(BeNil())
		})

		Context("constructing outputs", func() {
			It("constructs the put correctly", func() {
				Ω(fakeFactory.PutCallCount()).Should(Equal(1))

				workerID, delegate, resourceConfig, tags, params := fakeFactory.PutArgsForCall(0)
				Ω(workerID).Should(Equal(worker.Identifier{
					BuildID:      42,
					Type:         worker.ContainerTypePut,
					Name:         "some-output-resource",
					StepLocation: []uint{2, 0},
				}))
				Ω(delegate).Should(Equal(fakeOutputDelegate))
				Ω(resourceConfig.Name).Should(Equal("some-output-resource"))
				Ω(resourceConfig.Type).Should(Equal("some-type"))
				Ω(resourceConfig.Source).Should(Equal(atc.Source{"some": "source"}))
				Ω(tags).Should(ConsistOf("some", "putget", "tags"))
				Ω(params).Should(Equal(atc.Params{"some": "params"}))
			})

			It("constructs the dependent get correctly", func() {
				Ω(fakeFactory.DependentGetCallCount()).Should(Equal(1))

				sourceName, workerID, delegate, resourceConfig, tags, params := fakeFactory.DependentGetArgsForCall(0)
				Ω(workerID).Should(Equal(worker.Identifier{
					BuildID:      42,
					Type:         worker.ContainerTypeGet,
					Name:         "some-put",
					StepLocation: []uint{2, 1},
				}))
				Ω(tags).Should(ConsistOf("some", "putget", "tags"))

				Ω(delegate).Should(Equal(fakeInputDelegate))
				_, plan, location, substep := fakeDelegate.InputDelegateArgsForCall(1)
				Ω(plan).Should(Equal(outputPlan.Plan.PutGet.Head.Put.GetPlan()))
				Ω(location).Should(Equal(event.OriginLocation{2, 1}))
				Ω(substep).Should(BeTrue())

				Ω(sourceName).Should(Equal(exec.SourceName("some-put")))
				Ω(resourceConfig.Name).Should(Equal("some-output-resource"))
				Ω(resourceConfig.Type).Should(Equal("some-type"))
				Ω(resourceConfig.Source).Should(Equal(atc.Source{"some": "source"}))
				Ω(params).Should(Equal(atc.Params{"another": "params"}))
			})
		})

		Context("when the steps complete", func() {
			BeforeEach(func() {
				assertNotReleased := func(signals <-chan os.Signal, ready chan<- struct{}) error {
					defer GinkgoRecover()
					Consistently(inputStep.ReleaseCallCount).Should(BeZero())
					Consistently(taskStep.ReleaseCallCount).Should(BeZero())
					Consistently(outputStep.ReleaseCallCount).Should(BeZero())
					return nil
				}

				inputStep.RunStub = assertNotReleased
				taskStep.RunStub = assertNotReleased
				outputStep.RunStub = assertNotReleased
			})

			It("releases all sources", func() {
				Ω(inputStep.ReleaseCallCount()).Should(Equal(1))
				Ω(taskStep.ReleaseCallCount()).Should(Equal(1))
				Ω(outputStep.ReleaseCallCount()).Should(Equal(2)) // put + get
			})
		})

		Context("when the task is privileged", func() {
			BeforeEach(func() {
				privileged = true
			})

			It("constructs the task step privileged", func() {
				Ω(fakeFactory.TaskCallCount()).Should(Equal(1))

				_, _, _, privileged, _, _ := fakeFactory.TaskArgsForCall(0)
				Ω(privileged).Should(Equal(exec.Privileged(true)))
			})
		})

		Context("when the input succeeds", func() {
			BeforeEach(func() {
				inputStep.RunReturns(nil)
			})

			Context("when executing the task errors", func() {
				disaster := errors.New("oh no!")

				BeforeEach(func() {
					taskStep.RunReturns(disaster)
				})

				It("does not run any outputs", func() {
					Ω(outputStep.RunCallCount()).Should(BeZero())
				})

				It("finishes with error", func() {
					Ω(fakeDelegate.FinishCallCount()).Should(Equal(1))
					_, cbErr := fakeDelegate.FinishArgsForCall(0)
					Ω(cbErr).Should(MatchError(ContainSubstring(disaster.Error())))
				})
			})

			Context("when executing the task succeeds", func() {
				BeforeEach(func() {
					taskStep.RunReturns(nil)
					taskStep.ResultStub = successResult(true)
				})

				Context("when the output should perform on success", func() {
					BeforeEach(func() {
						outputPlan.Conditions = atc.Conditions{atc.ConditionSuccess}
					})

					It("runs the output", func() {
						Ω(outputStep.RunCallCount()).Should(Equal(1))
					})

					Context("when the output succeeds", func() {
						BeforeEach(func() {
							outputStep.RunReturns(nil)
						})

						It("finishes with success", func() {
							Ω(fakeDelegate.FinishCallCount()).Should(Equal(1))
							_, cbErr := fakeDelegate.FinishArgsForCall(0)
							Ω(cbErr).ShouldNot(HaveOccurred())
						})
					})

					Context("when the output fails", func() {
						disaster := errors.New("oh no!")

						BeforeEach(func() {
							outputStep.RunReturns(disaster)
						})

						It("finishes with the error", func() {
							Ω(fakeDelegate.FinishCallCount()).Should(Equal(1))
							_, cbErr := fakeDelegate.FinishArgsForCall(0)
							Ω(cbErr).Should(MatchError(ContainSubstring(disaster.Error())))
						})
					})
				})

				Context("when the output should perform on failure", func() {
					BeforeEach(func() {
						outputPlan.Conditions = atc.Conditions{atc.ConditionFailure}
					})

					It("does not run the output", func() {
						Ω(outputStep.RunCallCount()).Should(BeZero())
					})
				})

				Context("when the output should perform on success or failure", func() {
					BeforeEach(func() {
						outputPlan.Conditions = atc.Conditions{atc.ConditionSuccess, atc.ConditionFailure}
					})

					It("runs the output", func() {
						Ω(outputStep.RunCallCount()).Should(Equal(1))
					})

					Context("when the output succeeds", func() {
						BeforeEach(func() {
							outputStep.RunReturns(nil)
						})

						It("finishes with success", func() {
							Ω(fakeDelegate.FinishCallCount()).Should(Equal(1))
							_, cbErr := fakeDelegate.FinishArgsForCall(0)
							Ω(cbErr).ShouldNot(HaveOccurred())
						})
					})

					Context("when the output fails", func() {
						disaster := errors.New("oh no!")

						BeforeEach(func() {
							outputStep.RunReturns(disaster)
						})

						It("finishes with the error", func() {
							Ω(fakeDelegate.FinishCallCount()).Should(Equal(1))
							_, cbErr := fakeDelegate.FinishArgsForCall(0)
							Ω(cbErr).Should(MatchError(ContainSubstring(disaster.Error())))
						})
					})
				})

				Context("when the output has empty conditions", func() {
					BeforeEach(func() {
						outputPlan.Conditions = atc.Conditions{}
					})

					It("does not run the output", func() {
						Ω(outputStep.RunCallCount()).Should(BeZero())
					})
				})
			})

			Context("when executing the task fails", func() {
				BeforeEach(func() {
					taskStep.RunReturns(nil)
					taskStep.ResultStub = successResult(false)
				})

				Context("when the output should perform on success", func() {
					BeforeEach(func() {
						outputPlan.Conditions = atc.Conditions{atc.ConditionSuccess}
					})

					It("does not run the output", func() {
						Ω(outputStep.RunCallCount()).Should(BeZero())
					})
				})

				Context("when the output should perform on failure", func() {
					BeforeEach(func() {
						outputPlan.Conditions = atc.Conditions{atc.ConditionFailure}
					})

					It("runs the output", func() {
						Ω(outputStep.RunCallCount()).Should(Equal(1))
					})

					Context("when the output succeeds", func() {
						BeforeEach(func() {
							outputStep.RunReturns(nil)
						})

						It("finishes with success", func() {
							Ω(fakeDelegate.FinishCallCount()).Should(Equal(1))
							_, cbErr := fakeDelegate.FinishArgsForCall(0)
							Ω(cbErr).ShouldNot(HaveOccurred())
						})
					})

					Context("when the output fails", func() {
						disaster := errors.New("oh no!")

						BeforeEach(func() {
							outputStep.RunReturns(disaster)
						})

						It("finishes with the error", func() {
							Ω(fakeDelegate.FinishCallCount()).Should(Equal(1))
							_, cbErr := fakeDelegate.FinishArgsForCall(0)
							Ω(cbErr).Should(MatchError(ContainSubstring(disaster.Error())))
						})
					})
				})

				Context("when the output should perform on success or failure", func() {
					BeforeEach(func() {
						outputPlan.Conditions = atc.Conditions{atc.ConditionSuccess, atc.ConditionFailure}
					})

					It("runs the output", func() {
						Ω(outputStep.RunCallCount()).Should(Equal(1))
					})

					Context("when the output succeeds", func() {
						BeforeEach(func() {
							outputStep.RunReturns(nil)
						})

						It("finishes with success", func() {
							Ω(fakeDelegate.FinishCallCount()).Should(Equal(1))
							_, cbErr := fakeDelegate.FinishArgsForCall(0)
							Ω(cbErr).ShouldNot(HaveOccurred())
						})
					})

					Context("when the output fails", func() {
						disaster := errors.New("oh no!")

						BeforeEach(func() {
							outputStep.RunReturns(disaster)
						})

						It("finishes with the error", func() {
							Ω(fakeDelegate.FinishCallCount()).Should(Equal(1))
							_, cbErr := fakeDelegate.FinishArgsForCall(0)
							Ω(cbErr).Should(MatchError(ContainSubstring(disaster.Error())))
						})
					})
				})

				Context("when the output has empty conditions", func() {
					BeforeEach(func() {
						outputPlan.Conditions = atc.Conditions{}
					})

					It("does not run the output", func() {
						Ω(outputStep.RunCallCount()).Should(BeZero())
					})
				})
			})
		})

		Context("when an input errors", func() {
			disaster := errors.New("oh no!")

			BeforeEach(func() {
				inputStep.RunReturns(disaster)
			})

			It("does not run the task", func() {
				Ω(taskStep.RunCallCount()).Should(BeZero())
			})

			It("does not run any outputs", func() {
				Ω(outputStep.RunCallCount()).Should(BeZero())
			})

			It("finishes with the error", func() {
				Ω(fakeDelegate.FinishCallCount()).Should(Equal(1))
				_, cbErr := fakeDelegate.FinishArgsForCall(0)
				Ω(cbErr).Should(MatchError(ContainSubstring(disaster.Error())))
			})
		})
	})
})

func successResult(result exec.Success) func(dest interface{}) bool {
	return func(dest interface{}) bool {
		switch x := dest.(type) {
		case *exec.Success:
			*x = result
			return true

		default:
			return false
		}
	}
}
