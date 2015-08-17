package engine_test

import (
	"os"

	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/engine"
	"github.com/concourse/atc/engine/fakes"
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
			outputPlan atc.Plan
			privileged bool
			taskConfig *atc.TaskConfig

			build          engine.Build
			taskConfigPath string

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

			outputPlan = atc.Plan{
				Location: &atc.Location{},
				OnSuccess: &atc.OnSuccessPlan{
					Step: atc.Plan{
						Location: &atc.Location{},
						Put: &atc.PutPlan{
							Name:     "some-put",
							Resource: "some-output-resource",
							Tags:     []string{"some", "putget", "tags"},
							Type:     "some-type",
							Source:   atc.Source{"some": "source"},
							Params:   atc.Params{"some": "params"},
						},
					},
					Next: atc.Plan{
						Location: &atc.Location{},
						DependentGet: &atc.DependentGetPlan{
							Name:     "some-put",
							Resource: "some-output-resource",
							Tags:     []string{"some", "putget", "tags"},
							Type:     "some-type",
							Source:   atc.Source{"some": "source"},
							Params:   atc.Params{"another": "params"},
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
			inputStep.ResultStub = successResult(true)
			inputStepFactory.UsingReturns(inputStep)
			fakeFactory.GetReturns(inputStepFactory)

			taskStepFactory = new(execfakes.FakeStepFactory)
			taskStep = new(execfakes.FakeStep)
			taskStep.ResultStub = successResult(true)
			taskStepFactory.UsingReturns(taskStep)
			fakeFactory.TaskReturns(taskStepFactory)

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

		})

		Describe("with a putget in an aggregate", func() {
			BeforeEach(func() {
				outputPlan =
					atc.Plan{
						Aggregate: &atc.AggregatePlan{
							atc.Plan{
								Location: &atc.Location{},
								OnSuccess: &atc.OnSuccessPlan{
									Step: atc.Plan{
										Location: &atc.Location{},
										Put: &atc.PutPlan{
											Name:     "some-put",
											Resource: "some-output-resource",
											Type:     "some-type",
											Source:   atc.Source{"some": "source"},
											Params:   atc.Params{"some": "params"},
										},
									},
									Next: atc.Plan{
										Location: &atc.Location{},
										DependentGet: &atc.DependentGetPlan{
											Name:     "some-put",
											Resource: "some-output-resource",
											Type:     "some-type",
											Source:   atc.Source{"some": "source"},
											Params:   atc.Params{"another": "params"},
										},
									},
								},
							},
							atc.Plan{
								Location: &atc.Location{},
								OnSuccess: &atc.OnSuccessPlan{
									Step: atc.Plan{
										Location: &atc.Location{},
										Put: &atc.PutPlan{
											Name:     "some-put-2",
											Resource: "some-output-resource-2",
											Type:     "some-type-2",
											Source:   atc.Source{"some": "source-2"},
											Params:   atc.Params{"some": "params-2"},
										},
									},
									Next: atc.Plan{
										Location: &atc.Location{},
										DependentGet: &atc.DependentGetPlan{
											Name:     "some-put-2",
											Resource: "some-output-resource-2",
											Type:     "some-type-2",
											Source:   atc.Source{"some": "source-2"},
											Params:   atc.Params{"another": "params-2"},
										},
									},
								},
							},
						},
					}
			})

			Context("constructing outputs", func() {
				It("constructs the put correctly", func() {
					var err error
					build, err = execEngine.CreateBuild(buildModel, outputPlan)
					Ω(err).ShouldNot(HaveOccurred())

					build.Resume(logger)
					Ω(fakeFactory.PutCallCount()).Should(Equal(2))

					workerID, delegate, resourceConfig, tags, params := fakeFactory.PutArgsForCall(0)
					Ω(workerID).Should(Equal(worker.Identifier{
						BuildID: 42,
						Type:    worker.ContainerTypePut,
						Name:    "some-put",
					}))
					Ω(tags).Should(BeEmpty())
					Ω(delegate).Should(Equal(fakeOutputDelegate))
					Ω(resourceConfig.Name).Should(Equal("some-output-resource"))
					Ω(resourceConfig.Type).Should(Equal("some-type"))
					Ω(resourceConfig.Source).Should(Equal(atc.Source{"some": "source"}))
					Ω(params).Should(Equal(atc.Params{"some": "params"}))

					workerID, delegate, resourceConfig, tags, params = fakeFactory.PutArgsForCall(1)
					Ω(workerID).Should(Equal(worker.Identifier{
						BuildID: 42,
						Type:    worker.ContainerTypePut,
						Name:    "some-put-2",
					}))
					Ω(tags).Should(BeEmpty())
					Ω(delegate).Should(Equal(fakeOutputDelegate))
					Ω(resourceConfig.Name).Should(Equal("some-output-resource-2"))
					Ω(resourceConfig.Type).Should(Equal("some-type-2"))
					Ω(resourceConfig.Source).Should(Equal(atc.Source{"some": "source-2"}))
					Ω(params).Should(Equal(atc.Params{"some": "params-2"}))
				})
				It("constructs the dependent get correctly", func() {
					var err error
					build, err = execEngine.CreateBuild(buildModel, outputPlan)
					Ω(err).ShouldNot(HaveOccurred())

					build.Resume(logger)
					Ω(fakeFactory.DependentGetCallCount()).Should(Equal(2))

					sourceName, workerID, delegate, resourceConfig, tags, params := fakeFactory.DependentGetArgsForCall(0)
					Ω(workerID).Should(Equal(worker.Identifier{
						BuildID: 42,
						Type:    worker.ContainerTypeGet,
						Name:    "some-put",
					}))

					Ω(tags).Should(BeEmpty())
					Ω(delegate).Should(Equal(fakeInputDelegate))
					_, plan, location := fakeDelegate.InputDelegateArgsForCall(0)
					Ω(plan).Should(Equal((*outputPlan.Aggregate)[0].OnSuccess.Next.DependentGet.GetPlan()))
					Ω(location).ShouldNot(BeNil())

					Ω(sourceName).Should(Equal(exec.SourceName("some-put")))
					Ω(resourceConfig.Name).Should(Equal("some-output-resource"))
					Ω(resourceConfig.Type).Should(Equal("some-type"))
					Ω(resourceConfig.Source).Should(Equal(atc.Source{"some": "source"}))
					Ω(params).Should(Equal(atc.Params{"another": "params"}))
					sourceName, workerID, delegate, resourceConfig, tags, params = fakeFactory.DependentGetArgsForCall(1)
					Ω(workerID).Should(Equal(worker.Identifier{
						BuildID: 42,
						Type:    worker.ContainerTypeGet,
						Name:    "some-put-2",
					}))

					Ω(tags).Should(BeEmpty())
					Ω(delegate).Should(Equal(fakeInputDelegate))
					_, plan, location = fakeDelegate.InputDelegateArgsForCall(1)
					Ω(plan).Should(Equal((*outputPlan.Aggregate)[1].OnSuccess.Next.DependentGet.GetPlan()))
					Ω(location).ShouldNot(BeNil())

					Ω(sourceName).Should(Equal(exec.SourceName("some-put-2")))
					Ω(resourceConfig.Name).Should(Equal("some-output-resource-2"))
					Ω(resourceConfig.Type).Should(Equal("some-type-2"))
					Ω(resourceConfig.Source).Should(Equal(atc.Source{"some": "source-2"}))
					Ω(params).Should(Equal(atc.Params{"another": "params-2"}))
				})
			})
		})

		Context("with a basic plan", func() {
			Context("that contains inputs", func() {

				getPlan := &atc.GetPlan{
					Name:     "some-input",
					Resource: "some-input-resource",
					Type:     "some-type",
					Tags:     []string{"some", "get", "tags"},
					Version:  atc.Version{"some": "version"},
					Source:   atc.Source{"some": "source"},
					Params:   atc.Params{"some": "params"},
				}

				plan := atc.Plan{
					Location: &atc.Location{
						ID:       145,
						ParentID: 1,

						ParallelGroup: 1234,
						SerialGroup:   5678,

						Hook: "boring input hook",
					},
					Get: getPlan,
				}

				It("constructs inputs correctly", func() {
					var err error
					build, err := execEngine.CreateBuild(buildModel, plan)
					Ω(err).ShouldNot(HaveOccurred())

					build.Resume(logger)
					Ω(fakeFactory.GetCallCount()).Should(Equal(1))

					sourceName, workerID, delegate, resourceConfig, params, tags, version := fakeFactory.GetArgsForCall(0)
					Ω(sourceName).Should(Equal(exec.SourceName("some-input")))
					Ω(workerID).Should(Equal(worker.Identifier{
						BuildID:      42,
						Type:         worker.ContainerTypeGet,
						Name:         "some-input",
						StepLocation: 145,
					}))
					Ω(tags).Should(ConsistOf("some", "get", "tags"))
					Ω(resourceConfig.Name).Should(Equal("some-input-resource"))
					Ω(resourceConfig.Type).Should(Equal("some-type"))
					Ω(resourceConfig.Source).Should(Equal(atc.Source{"some": "source"}))
					Ω(params).Should(Equal(atc.Params{"some": "params"}))
					Ω(version).Should(Equal(atc.Version{"some": "version"}))

					Ω(delegate).Should(Equal(fakeInputDelegate))
					_, plan, location := fakeDelegate.InputDelegateArgsForCall(0)
					Ω(plan).Should(Equal(*inputPlan))
					Ω(location).ShouldNot(BeNil())
					Ω(location.ID).Should(Equal(uint(145)))
					Ω(location.ParentID).Should(Equal(uint(1)))
					Ω(location.ParallelGroup).Should(Equal(uint(1234)))
					Ω(location.SerialGroup).Should(Equal(uint(5678)))
					Ω(location.Hook).Should(Equal("boring input hook"))

				})

				It("releases inputs correctly", func() {
					inputStep.RunStub = func(signals <-chan os.Signal, ready chan<- struct{}) error {
						defer GinkgoRecover()
						Consistently(inputStep.ReleaseCallCount).Should(BeZero())
						return nil
					}
					var err error
					build, err = execEngine.CreateBuild(buildModel, plan)
					Ω(err).ShouldNot(HaveOccurred())
					build.Resume(logger)

					Ω(inputStep.ReleaseCallCount()).Should(Equal(1))
				})
			})

			Context("that contains tasks", func() {
				privileged = false

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

				taskPlan := &atc.TaskPlan{
					Name:       "some-task",
					Config:     taskConfig,
					ConfigPath: taskConfigPath,
					Privileged: privileged,
				}
				plan := atc.Plan{
					Location: &atc.Location{
						ID:       123,
						ParentID: 41,

						ParallelGroup: 123498,
						SerialGroup:   69,

						Hook: "look at me I'm a hook",
					},
					Task: taskPlan,
				}
				It("constructs tasks correctly", func() {
					var err error
					build, err = execEngine.CreateBuild(buildModel, plan)
					Ω(err).ShouldNot(HaveOccurred())

					build.Resume(logger)
					Ω(fakeFactory.TaskCallCount()).Should(Equal(1))

					sourceName, workerID, delegate, privileged, tags, configSource := fakeFactory.TaskArgsForCall(0)
					Ω(sourceName).Should(Equal(exec.SourceName("some-task")))
					Ω(workerID).Should(Equal(worker.Identifier{
						BuildID:      42,
						Type:         worker.ContainerTypeTask,
						Name:         "some-task",
						StepLocation: 123,
					}))
					Ω(privileged).Should(Equal(exec.Privileged(false)))
					Ω(tags).Should(BeEmpty())
					Ω(configSource).ShouldNot(BeNil())

					Ω(delegate).Should(Equal(fakeExecutionDelegate))
					_, _, location := fakeDelegate.ExecutionDelegateArgsForCall(0)

					Ω(location).ShouldNot(BeNil())
					Ω(location.ID).Should(Equal(uint(123)))
					Ω(location.ParentID).Should(Equal(uint(41)))
					Ω(location.ParallelGroup).Should(Equal(uint(123498)))
					Ω(location.SerialGroup).Should(Equal(uint(69)))
					Ω(location.Hook).Should(Equal("look at me I'm a hook"))
				})

				It("releases the tasks correctly", func() {
					taskStep.RunStub = func(signals <-chan os.Signal, ready chan<- struct{}) error {
						defer GinkgoRecover()
						Consistently(taskStep.ReleaseCallCount).Should(BeZero())
						return nil
					}
					var err error
					build, err = execEngine.CreateBuild(buildModel, plan)
					Ω(err).ShouldNot(HaveOccurred())
					build.Resume(logger)

					Ω(taskStep.ReleaseCallCount()).Should(Equal(1))

				})

				Context("when the task is privileged", func() {
					BeforeEach(func() {
						taskPlan.Privileged = true
					})

					It("constructs the task step privileged", func() {
						var err error
						build, err = execEngine.CreateBuild(buildModel, plan)
						Ω(err).ShouldNot(HaveOccurred())

						build.Resume(logger)
						Ω(fakeFactory.TaskCallCount()).Should(Equal(1))

						_, _, _, privileged, _, _ := fakeFactory.TaskArgsForCall(0)
						Ω(privileged).Should(Equal(exec.Privileged(true)))
					})
				})

			})

			Context("that contains outputs", func() {
				plan := atc.Plan{
					Location: &atc.Location{

						ID:       50,
						ParentID: 25,

						ParallelGroup: 1020,
						SerialGroup:   150,

						Hook: "hook",
					},

					OnSuccess: &atc.OnSuccessPlan{
						Step: atc.Plan{
							Location: &atc.Location{

								ID:       51,
								ParentID: 26,

								ParallelGroup: 1021,
								SerialGroup:   151,

								Hook: "special hook",
							},
							Put: &atc.PutPlan{
								Name:     "some-put",
								Resource: "some-output-resource",
								Tags:     []string{"some", "putget", "tags"},
								Type:     "some-type",
								Source:   atc.Source{"some": "source"},
								Params:   atc.Params{"some": "params"},
							},
						},
						Next: atc.Plan{
							Location: &atc.Location{

								ID:       512,
								ParentID: 2134,

								ParallelGroup: 12,
								SerialGroup:   121243,

								Hook: "more special hook",
							},
							DependentGet: &atc.DependentGetPlan{
								Name:     "some-put",
								Resource: "some-output-resource",
								Tags:     []string{"some", "putget", "tags"},
								Type:     "some-type",
								Source:   atc.Source{"some": "source"},
								Params:   atc.Params{"another": "params"},
							},
						},
					},
				}

				It("constructs the put correctly", func() {
					var err error
					build, err = execEngine.CreateBuild(buildModel, plan)
					Ω(err).ShouldNot(HaveOccurred())

					build.Resume(logger)
					Ω(fakeFactory.PutCallCount()).Should(Equal(1))

					workerID, delegate, resourceConfig, tags, params := fakeFactory.PutArgsForCall(0)
					Ω(workerID).Should(Equal(worker.Identifier{
						BuildID:      42,
						Type:         worker.ContainerTypePut,
						Name:         "some-put",
						StepLocation: 51,
					}))
					Ω(resourceConfig.Name).Should(Equal("some-output-resource"))
					Ω(resourceConfig.Type).Should(Equal("some-type"))
					Ω(resourceConfig.Source).Should(Equal(atc.Source{"some": "source"}))
					Ω(tags).Should(ConsistOf("some", "putget", "tags"))
					Ω(params).Should(Equal(atc.Params{"some": "params"}))

					Ω(delegate).Should(Equal(fakeOutputDelegate))
					_, _, location := fakeDelegate.OutputDelegateArgsForCall(0)

					Ω(location).ShouldNot(BeNil())
					Ω(location.ID).Should(Equal(uint(51)))
					Ω(location.ParentID).Should(Equal(uint(26)))
					Ω(location.ParallelGroup).Should(Equal(uint(1021)))
					Ω(location.SerialGroup).Should(Equal(uint(151)))
					Ω(location.Hook).Should(Equal("special hook"))
				})

				It("constructs the dependent get correctly", func() {
					var err error
					build, err = execEngine.CreateBuild(buildModel, plan)
					Ω(err).ShouldNot(HaveOccurred())

					build.Resume(logger)
					Ω(fakeFactory.DependentGetCallCount()).Should(Equal(1))

					sourceName, workerID, delegate, resourceConfig, tags, params := fakeFactory.DependentGetArgsForCall(0)
					Ω(workerID).Should(Equal(worker.Identifier{
						BuildID:      42,
						Type:         worker.ContainerTypeGet,
						Name:         "some-put",
						StepLocation: 512,
					}))
					Ω(tags).Should(ConsistOf("some", "putget", "tags"))
					Ω(sourceName).Should(Equal(exec.SourceName("some-put")))
					Ω(resourceConfig.Name).Should(Equal("some-output-resource"))
					Ω(resourceConfig.Type).Should(Equal("some-type"))
					Ω(resourceConfig.Source).Should(Equal(atc.Source{"some": "source"}))
					Ω(params).Should(Equal(atc.Params{"another": "params"}))

					Ω(delegate).Should(Equal(fakeInputDelegate))
					_, _, location := fakeDelegate.InputDelegateArgsForCall(0)
					Ω(location).ShouldNot(BeNil())
					Ω(location.ID).Should(Equal(uint(512)))
					Ω(location.ParentID).Should(Equal(uint(2134)))
					Ω(location.ParallelGroup).Should(Equal(uint(12)))
					Ω(location.SerialGroup).Should(Equal(uint(121243)))
					Ω(location.Hook).Should(Equal("more special hook"))
				})
				It("releases all sources", func() {
					var err error
					build, err = execEngine.CreateBuild(buildModel, plan)
					Ω(err).ShouldNot(HaveOccurred())

					build.Resume(logger)
					Ω(outputStep.ReleaseCallCount()).Should(Equal(1))
					Ω(dependentStep.ReleaseCallCount()).Should(Equal(1))

				})

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
