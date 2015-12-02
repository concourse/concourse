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

			buildModel       db.Build
			expectedMetadata engine.StepMetadata

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

			buildModel = db.Build{
				ID:           42,
				Name:         "21",
				JobName:      "some-job",
				PipelineName: "some-pipeline",
			}

			expectedMetadata = engine.StepMetadata{
				BuildID:      42,
				BuildName:    "21",
				JobName:      "some-job",
				PipelineName: "some-pipeline",
			}

			taskConfig = &atc.TaskConfig{
				Image:  "some-image",
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
				Pipeline: "some-pipeline",
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
							Pipeline: "some-pipeline",
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
											Pipeline: "some-pipeline",
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
											Pipeline: "some-pipeline",
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
											Pipeline: "some-pipeline",
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
											Pipeline: "some-pipeline",
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
					build, err = execEngine.CreateBuild(logger, buildModel, outputPlan)
					Expect(err).NotTo(HaveOccurred())

					build.Resume(logger)
					Expect(fakeFactory.PutCallCount()).To(Equal(2))

					logger, metadata, workerID, delegate, resourceConfig, tags, params := fakeFactory.PutArgsForCall(0)
					Expect(logger).NotTo(BeNil())
					Expect(metadata).To(Equal(expectedMetadata))
					Expect(workerID).To(Equal(worker.Identifier{
						BuildID:      42,
						Type:         db.ContainerTypePut,
						Name:         "some-put",
						PipelineName: "some-pipeline",
					}))

					Expect(tags).To(BeEmpty())
					Expect(delegate).To(Equal(fakeOutputDelegate))
					Expect(resourceConfig.Name).To(Equal("some-output-resource"))
					Expect(resourceConfig.Type).To(Equal("some-type"))
					Expect(resourceConfig.Source).To(Equal(atc.Source{"some": "source"}))
					Expect(params).To(Equal(atc.Params{"some": "params"}))

					logger, metadata, workerID, delegate, resourceConfig, tags, params = fakeFactory.PutArgsForCall(1)
					Expect(logger).NotTo(BeNil())
					Expect(metadata).To(Equal(expectedMetadata))
					Expect(workerID).To(Equal(worker.Identifier{
						BuildID:      42,
						Type:         db.ContainerTypePut,
						Name:         "some-put-2",
						PipelineName: "some-pipeline",
					}))

					Expect(tags).To(BeEmpty())
					Expect(delegate).To(Equal(fakeOutputDelegate))
					Expect(resourceConfig.Name).To(Equal("some-output-resource-2"))
					Expect(resourceConfig.Type).To(Equal("some-type-2"))
					Expect(resourceConfig.Source).To(Equal(atc.Source{"some": "source-2"}))
					Expect(params).To(Equal(atc.Params{"some": "params-2"}))
				})

				It("constructs the dependent get correctly", func() {
					var err error
					build, err = execEngine.CreateBuild(logger, buildModel, outputPlan)
					Expect(err).NotTo(HaveOccurred())

					build.Resume(logger)
					Expect(fakeFactory.DependentGetCallCount()).To(Equal(2))

					logger, metadata, sourceName, workerID, delegate, resourceConfig, tags, params := fakeFactory.DependentGetArgsForCall(0)
					Expect(logger).NotTo(BeNil())
					Expect(metadata).To(Equal(expectedMetadata))
					Expect(workerID).To(Equal(worker.Identifier{
						BuildID:      42,
						Type:         db.ContainerTypeGet,
						Name:         "some-put",
						PipelineName: "some-pipeline",
					}))

					Expect(tags).To(BeEmpty())
					Expect(delegate).To(Equal(fakeInputDelegate))
					_, plan, location := fakeDelegate.InputDelegateArgsForCall(0)
					Expect(plan).To(Equal((*outputPlan.Aggregate)[0].OnSuccess.Next.DependentGet.GetPlan()))
					Expect(location).NotTo(BeNil())

					Expect(sourceName).To(Equal(exec.SourceName("some-put")))
					Expect(resourceConfig.Name).To(Equal("some-output-resource"))
					Expect(resourceConfig.Type).To(Equal("some-type"))
					Expect(resourceConfig.Source).To(Equal(atc.Source{"some": "source"}))
					Expect(params).To(Equal(atc.Params{"another": "params"}))

					logger, metadata, sourceName, workerID, delegate, resourceConfig, tags, params = fakeFactory.DependentGetArgsForCall(1)
					Expect(logger).NotTo(BeNil())
					Expect(metadata).To(Equal(expectedMetadata))
					Expect(workerID).To(Equal(worker.Identifier{
						BuildID:      42,
						Type:         db.ContainerTypeGet,
						Name:         "some-put-2",
						PipelineName: "some-pipeline",
					}))

					Expect(tags).To(BeEmpty())
					Expect(delegate).To(Equal(fakeInputDelegate))
					_, plan, location = fakeDelegate.InputDelegateArgsForCall(1)
					Expect(plan).To(Equal((*outputPlan.Aggregate)[1].OnSuccess.Next.DependentGet.GetPlan()))
					Expect(location).NotTo(BeNil())

					Expect(sourceName).To(Equal(exec.SourceName("some-put-2")))
					Expect(resourceConfig.Name).To(Equal("some-output-resource-2"))
					Expect(resourceConfig.Type).To(Equal("some-type-2"))
					Expect(resourceConfig.Source).To(Equal(atc.Source{"some": "source-2"}))
					Expect(params).To(Equal(atc.Params{"another": "params-2"}))
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
					Pipeline: "some-pipeline",
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
					build, err := execEngine.CreateBuild(logger, buildModel, plan)
					Expect(err).NotTo(HaveOccurred())

					build.Resume(logger)
					Expect(fakeFactory.GetCallCount()).To(Equal(1))

					logger, metadata, sourceName, workerID, delegate, resourceConfig, params, tags, version := fakeFactory.GetArgsForCall(0)
					Expect(logger).NotTo(BeNil())
					Expect(metadata).To(Equal(expectedMetadata))
					Expect(sourceName).To(Equal(exec.SourceName("some-input")))
					Expect(workerID).To(Equal(worker.Identifier{
						BuildID:      42,
						Type:         db.ContainerTypeGet,
						Name:         "some-input",
						PipelineName: "some-pipeline",
						StepLocation: 145,
					}))

					Expect(tags).To(ConsistOf("some", "get", "tags"))
					Expect(resourceConfig.Name).To(Equal("some-input-resource"))
					Expect(resourceConfig.Type).To(Equal("some-type"))
					Expect(resourceConfig.Source).To(Equal(atc.Source{"some": "source"}))
					Expect(params).To(Equal(atc.Params{"some": "params"}))
					Expect(version).To(Equal(atc.Version{"some": "version"}))

					Expect(delegate).To(Equal(fakeInputDelegate))
					_, _, location := fakeDelegate.InputDelegateArgsForCall(0)
					Expect(location).NotTo(BeNil())
					Expect(location.ID).To(Equal(uint(145)))
					Expect(location.ParentID).To(Equal(uint(1)))
					Expect(location.ParallelGroup).To(Equal(uint(1234)))
					Expect(location.SerialGroup).To(Equal(uint(5678)))
					Expect(location.Hook).To(Equal("boring input hook"))

				})

				It("releases inputs correctly", func() {
					inputStep.RunStub = func(signals <-chan os.Signal, ready chan<- struct{}) error {
						defer GinkgoRecover()
						Consistently(inputStep.ReleaseCallCount).Should(BeZero())
						return nil
					}
					var err error
					build, err = execEngine.CreateBuild(logger, buildModel, plan)
					Expect(err).NotTo(HaveOccurred())
					build.Resume(logger)

					Expect(inputStep.ReleaseCallCount()).To(Equal(1))
				})
			})

			Context("that contains tasks", func() {
				privileged = false

				taskConfig = &atc.TaskConfig{
					Image:  "some-image",
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
					Pipeline:   "some-pipeline",
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
					build, err = execEngine.CreateBuild(logger, buildModel, plan)
					Expect(err).NotTo(HaveOccurred())

					build.Resume(logger)
					Expect(fakeFactory.TaskCallCount()).To(Equal(1))

					logger, sourceName, workerID, delegate, privileged, tags, configSource := fakeFactory.TaskArgsForCall(0)
					Expect(logger).NotTo(BeNil())
					Expect(sourceName).To(Equal(exec.SourceName("some-task")))
					Expect(workerID).To(Equal(worker.Identifier{
						BuildID:      42,
						Type:         db.ContainerTypeTask,
						Name:         "some-task",
						PipelineName: "some-pipeline",
						StepLocation: 123,
					}))

					Expect(privileged).To(Equal(exec.Privileged(false)))
					Expect(tags).To(BeEmpty())
					Expect(configSource).NotTo(BeNil())

					Expect(delegate).To(Equal(fakeExecutionDelegate))
					_, _, location := fakeDelegate.ExecutionDelegateArgsForCall(0)

					Expect(location).NotTo(BeNil())
					Expect(location.ID).To(Equal(uint(123)))
					Expect(location.ParentID).To(Equal(uint(41)))
					Expect(location.ParallelGroup).To(Equal(uint(123498)))
					Expect(location.SerialGroup).To(Equal(uint(69)))
					Expect(location.Hook).To(Equal("look at me I'm a hook"))
				})

				It("releases the tasks correctly", func() {
					taskStep.RunStub = func(signals <-chan os.Signal, ready chan<- struct{}) error {
						defer GinkgoRecover()
						Consistently(taskStep.ReleaseCallCount).Should(BeZero())
						return nil
					}
					var err error
					build, err = execEngine.CreateBuild(logger, buildModel, plan)
					Expect(err).NotTo(HaveOccurred())
					build.Resume(logger)

					Expect(taskStep.ReleaseCallCount()).To(Equal(1))
				})

				Context("when the task is privileged", func() {
					BeforeEach(func() {
						taskPlan.Privileged = true
					})

					It("constructs the task step privileged", func() {
						var err error
						build, err = execEngine.CreateBuild(logger, buildModel, plan)
						Expect(err).NotTo(HaveOccurred())

						build.Resume(logger)
						Expect(fakeFactory.TaskCallCount()).To(Equal(1))

						_, _, _, _, privileged, _, _ := fakeFactory.TaskArgsForCall(0)
						Expect(privileged).To(Equal(exec.Privileged(true)))
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
								Pipeline: "some-pipeline",
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
								Pipeline: "some-pipeline",
							},
						},
					},
				}

				It("constructs the put correctly", func() {
					var err error
					build, err = execEngine.CreateBuild(logger, buildModel, plan)
					Expect(err).NotTo(HaveOccurred())

					build.Resume(logger)
					Expect(fakeFactory.PutCallCount()).To(Equal(1))

					logger, metadata, workerID, delegate, resourceConfig, tags, params := fakeFactory.PutArgsForCall(0)
					Expect(logger).NotTo(BeNil())
					Expect(metadata).To(Equal(expectedMetadata))
					Expect(workerID).To(Equal(worker.Identifier{
						BuildID:      42,
						Type:         db.ContainerTypePut,
						Name:         "some-put",
						PipelineName: "some-pipeline",
						StepLocation: 51,
					}))

					Expect(resourceConfig.Name).To(Equal("some-output-resource"))
					Expect(resourceConfig.Type).To(Equal("some-type"))
					Expect(resourceConfig.Source).To(Equal(atc.Source{"some": "source"}))
					Expect(tags).To(ConsistOf("some", "putget", "tags"))
					Expect(params).To(Equal(atc.Params{"some": "params"}))

					Expect(delegate).To(Equal(fakeOutputDelegate))
					_, _, location := fakeDelegate.OutputDelegateArgsForCall(0)

					Expect(location).NotTo(BeNil())
					Expect(location.ID).To(Equal(uint(51)))
					Expect(location.ParentID).To(Equal(uint(26)))
					Expect(location.ParallelGroup).To(Equal(uint(1021)))
					Expect(location.SerialGroup).To(Equal(uint(151)))
					Expect(location.Hook).To(Equal("special hook"))
				})

				It("constructs the dependent get correctly", func() {
					var err error
					build, err = execEngine.CreateBuild(logger, buildModel, plan)
					Expect(err).NotTo(HaveOccurred())

					build.Resume(logger)
					Expect(fakeFactory.DependentGetCallCount()).To(Equal(1))

					logger, metadata, sourceName, workerID, delegate, resourceConfig, tags, params := fakeFactory.DependentGetArgsForCall(0)
					Expect(logger).NotTo(BeNil())
					Expect(metadata).To(Equal(expectedMetadata))
					Expect(workerID).To(Equal(worker.Identifier{
						BuildID:      42,
						Type:         db.ContainerTypeGet,
						Name:         "some-put",
						PipelineName: "some-pipeline",
						StepLocation: 512,
					}))

					Expect(tags).To(ConsistOf("some", "putget", "tags"))
					Expect(sourceName).To(Equal(exec.SourceName("some-put")))
					Expect(resourceConfig.Name).To(Equal("some-output-resource"))
					Expect(resourceConfig.Type).To(Equal("some-type"))
					Expect(resourceConfig.Source).To(Equal(atc.Source{"some": "source"}))
					Expect(params).To(Equal(atc.Params{"another": "params"}))

					Expect(delegate).To(Equal(fakeInputDelegate))
					_, _, location := fakeDelegate.InputDelegateArgsForCall(0)
					Expect(location).NotTo(BeNil())
					Expect(location.ID).To(Equal(uint(512)))
					Expect(location.ParentID).To(Equal(uint(2134)))
					Expect(location.ParallelGroup).To(Equal(uint(12)))
					Expect(location.SerialGroup).To(Equal(uint(121243)))
					Expect(location.Hook).To(Equal("more special hook"))
				})
				It("releases all sources", func() {
					var err error
					build, err = execEngine.CreateBuild(logger, buildModel, plan)
					Expect(err).NotTo(HaveOccurred())

					build.Resume(logger)
					Expect(outputStep.ReleaseCallCount()).To(Equal(1))
					Expect(dependentStep.ReleaseCallCount()).To(Equal(1))

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
