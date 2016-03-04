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
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pivotal-golang/lager"
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

		execEngine = engine.NewExecEngine(fakeFactory, fakeDelegateFactory, fakeDB, "http://example.com")
	})

	Describe("Resume", func() {
		var (
			fakeDelegate          *fakes.FakeBuildDelegate
			fakeInputDelegate     *execfakes.FakeGetDelegate
			fakeExecutionDelegate *execfakes.FakeTaskDelegate
			fakeOutputDelegate    *execfakes.FakePutDelegate

			buildModel       db.Build
			expectedMetadata engine.StepMetadata

			outputPlan atc.Plan

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

			planFactory atc.PlanFactory
		)

		BeforeEach(func() {
			logger = lagertest.NewTestLogger("test")
			planFactory = atc.NewPlanFactory(123)

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
				ExternalURL:  "http://example.com",
			}

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
			var (
				putPlan               atc.Plan
				dependentGetPlan      atc.Plan
				otherPutPlan          atc.Plan
				otherDependentGetPlan atc.Plan
			)

			BeforeEach(func() {
				putPlan = planFactory.NewPlan(atc.PutPlan{
					Name:     "some-put",
					Resource: "some-output-resource",
					Type:     "put",
					Source:   atc.Source{"some": "source"},
					Params:   atc.Params{"some": "params"},
					Pipeline: "some-pipeline",
				})
				dependentGetPlan = planFactory.NewPlan(atc.DependentGetPlan{
					Name:     "some-get",
					Resource: "some-input-resource",
					Type:     "get",
					Source:   atc.Source{"some": "source"},
					Params:   atc.Params{"another": "params"},
					Pipeline: "some-pipeline",
				})

				otherPutPlan = planFactory.NewPlan(atc.PutPlan{
					Name:     "some-put-2",
					Resource: "some-output-resource-2",
					Type:     "put",
					Source:   atc.Source{"some": "source-2"},
					Params:   atc.Params{"some": "params-2"},
					Pipeline: "some-pipeline",
				})
				otherDependentGetPlan = planFactory.NewPlan(atc.DependentGetPlan{
					Name:     "some-get-2",
					Resource: "some-input-resource-2",
					Type:     "get",
					Source:   atc.Source{"some": "source-2"},
					Params:   atc.Params{"another": "params-2"},
					Pipeline: "some-pipeline",
				})

				outputPlan = planFactory.NewPlan(atc.AggregatePlan{
					planFactory.NewPlan(atc.OnSuccessPlan{
						Step: putPlan,
						Next: dependentGetPlan,
					}),
					planFactory.NewPlan(atc.OnSuccessPlan{
						Step: otherPutPlan,
						Next: otherDependentGetPlan,
					}),
				})
			})

			Context("constructing outputs", func() {
				It("constructs the put correctly", func() {
					var err error
					build, err = execEngine.CreateBuild(logger, buildModel, outputPlan)
					Expect(err).NotTo(HaveOccurred())

					build.Resume(logger)
					Expect(fakeFactory.PutCallCount()).To(Equal(2))

					logger, metadata, workerID, workerMetadata, delegate, resourceConfig, tags, params, _ := fakeFactory.PutArgsForCall(0)
					Expect(logger).NotTo(BeNil())
					Expect(metadata).To(Equal(expectedMetadata))
					Expect(workerMetadata).To(Equal(worker.Metadata{
						ResourceName: "",
						Type:         db.ContainerTypePut,
						StepName:     "some-put",
						PipelineName: "some-pipeline",
					}))
					Expect(workerID).To(Equal(worker.Identifier{
						BuildID: 42,
						PlanID:  putPlan.ID,
					}))

					Expect(tags).To(BeEmpty())
					Expect(delegate).To(Equal(fakeOutputDelegate))
					Expect(resourceConfig.Name).To(Equal("some-output-resource"))
					Expect(resourceConfig.Type).To(Equal("put"))
					Expect(resourceConfig.Source).To(Equal(atc.Source{"some": "source"}))
					Expect(params).To(Equal(atc.Params{"some": "params"}))

					logger, metadata, workerID, workerMetadata, delegate, resourceConfig, tags, params, _ = fakeFactory.PutArgsForCall(1)
					Expect(logger).NotTo(BeNil())
					Expect(metadata).To(Equal(expectedMetadata))
					Expect(workerMetadata).To(Equal(worker.Metadata{
						ResourceName: "",
						Type:         db.ContainerTypePut,
						StepName:     "some-put-2",
						PipelineName: "some-pipeline",
					}))
					Expect(workerID).To(Equal(worker.Identifier{
						BuildID: 42,
						PlanID:  otherPutPlan.ID,
					}))

					Expect(tags).To(BeEmpty())
					Expect(delegate).To(Equal(fakeOutputDelegate))
					Expect(resourceConfig.Name).To(Equal("some-output-resource-2"))
					Expect(resourceConfig.Type).To(Equal("put"))
					Expect(resourceConfig.Source).To(Equal(atc.Source{"some": "source-2"}))
					Expect(params).To(Equal(atc.Params{"some": "params-2"}))
				})

				It("constructs the dependent get correctly", func() {
					var err error
					build, err = execEngine.CreateBuild(logger, buildModel, outputPlan)
					Expect(err).NotTo(HaveOccurred())

					build.Resume(logger)
					Expect(fakeFactory.DependentGetCallCount()).To(Equal(2))

					logger, metadata, sourceName, workerID, workerMetadata, delegate, resourceConfig, tags, params, _ := fakeFactory.DependentGetArgsForCall(0)
					Expect(logger).NotTo(BeNil())
					Expect(metadata).To(Equal(expectedMetadata))
					Expect(workerMetadata).To(Equal(worker.Metadata{
						ResourceName: "",
						Type:         db.ContainerTypeGet,
						StepName:     "some-get",
						PipelineName: "some-pipeline",
					}))
					Expect(workerID).To(Equal(worker.Identifier{
						BuildID: 42,
						PlanID:  dependentGetPlan.ID,
					}))

					Expect(tags).To(BeEmpty())
					Expect(delegate).To(Equal(fakeInputDelegate))
					_, plan, planID := fakeDelegate.InputDelegateArgsForCall(0)
					Expect(plan).To(Equal((*outputPlan.Aggregate)[0].OnSuccess.Next.DependentGet.GetPlan()))
					Expect(planID).NotTo(BeNil())

					Expect(sourceName).To(Equal(exec.SourceName("some-get")))
					Expect(resourceConfig.Name).To(Equal("some-input-resource"))
					Expect(resourceConfig.Type).To(Equal("get"))
					Expect(resourceConfig.Source).To(Equal(atc.Source{"some": "source"}))
					Expect(params).To(Equal(atc.Params{"another": "params"}))

					logger, metadata, sourceName, workerID, workerMetadata, delegate, resourceConfig, tags, params, _ = fakeFactory.DependentGetArgsForCall(1)
					Expect(logger).NotTo(BeNil())
					Expect(metadata).To(Equal(expectedMetadata))
					Expect(workerMetadata).To(Equal(worker.Metadata{
						ResourceName: "",
						Type:         db.ContainerTypeGet,
						StepName:     "some-get-2",
						PipelineName: "some-pipeline",
					}))
					Expect(workerID).To(Equal(worker.Identifier{
						BuildID: 42,
						PlanID:  otherDependentGetPlan.ID,
					}))

					Expect(tags).To(BeEmpty())
					Expect(delegate).To(Equal(fakeInputDelegate))
					_, plan, planID = fakeDelegate.InputDelegateArgsForCall(1)
					Expect(plan).To(Equal((*outputPlan.Aggregate)[1].OnSuccess.Next.DependentGet.GetPlan()))
					Expect(planID).NotTo(BeNil())

					Expect(sourceName).To(Equal(exec.SourceName("some-get-2")))
					Expect(resourceConfig.Name).To(Equal("some-input-resource-2"))
					Expect(resourceConfig.Type).To(Equal("get"))
					Expect(resourceConfig.Source).To(Equal(atc.Source{"some": "source-2"}))
					Expect(params).To(Equal(atc.Params{"another": "params-2"}))
				})
			})
		})

		Context("with a retry plan", func() {
			var (
				getPlan       atc.Plan
				taskPlan      atc.Plan
				aggregatePlan atc.Plan
				doPlan        atc.Plan
				timeoutPlan   atc.Plan
				tryPlan       atc.Plan
				retryPlan     atc.Plan
				retryPlanTwo  atc.Plan
				err           error
			)
			BeforeEach(func() {
				getPlan = planFactory.NewPlan(atc.GetPlan{
					Name:     "some-get",
					Resource: "some-input-resource",
					Type:     "get",
					Source:   atc.Source{"some": "source"},
					Params:   atc.Params{"some": "params"},
					Pipeline: "some-pipeline",
				})

				taskPlan = planFactory.NewPlan(atc.TaskPlan{
					Name:       "some-task",
					Privileged: false,
					Tags:       atc.Tags{"some", "task", "tags"},
					Pipeline:   "some-pipeline",
					ConfigPath: "some-config-path",
				})

				retryPlanTwo = planFactory.NewPlan(atc.RetryPlan{
					taskPlan,
					taskPlan,
				})

				aggregatePlan = planFactory.NewPlan(atc.AggregatePlan{retryPlanTwo})

				doPlan = planFactory.NewPlan(atc.DoPlan{aggregatePlan})

				timeoutPlan = planFactory.NewPlan(atc.TimeoutPlan{
					Step:     doPlan,
					Duration: "1m",
				})

				tryPlan = planFactory.NewPlan(atc.TryPlan{
					Step: timeoutPlan,
				})

				retryPlan = planFactory.NewPlan(atc.RetryPlan{
					getPlan,
					timeoutPlan,
					getPlan,
				})

				build, err = execEngine.CreateBuild(logger, buildModel, retryPlan)
				Expect(err).NotTo(HaveOccurred())
				build.Resume(logger)
				Expect(fakeFactory.GetCallCount()).To(Equal(2))
				Expect(fakeFactory.TaskCallCount()).To(Equal(2))
			})

			It("constructs the retry correctly", func() {
				Expect(*retryPlan.Retry).To(HaveLen(3))
			})

			It("constructss the first get correctly", func() {
				logger, metadata, sourceName, workerID, workerMetadata, delegate, resourceConfig, tags, params, _, _ := fakeFactory.GetArgsForCall(0)
				Expect(logger).NotTo(BeNil())
				Expect(metadata).To(Equal(expectedMetadata))
				Expect(workerMetadata).To(Equal(worker.Metadata{
					ResourceName: "",
					Type:         db.ContainerTypeGet,
					StepName:     "some-get",
					PipelineName: "some-pipeline",
					Attempts:     []int{1},
				}))
				Expect(workerID).To(Equal(worker.Identifier{
					BuildID: 42,
					PlanID:  getPlan.ID,
				}))

				Expect(tags).To(BeEmpty())
				Expect(delegate).To(Equal(fakeInputDelegate))

				Expect(sourceName).To(Equal(exec.SourceName("some-get")))
				Expect(resourceConfig.Name).To(Equal("some-input-resource"))
				Expect(resourceConfig.Type).To(Equal("get"))
				Expect(resourceConfig.Source).To(Equal(atc.Source{"some": "source"}))
				Expect(params).To(Equal(atc.Params{"some": "params"}))
			})

			It("constructs the second get correctly", func() {
				logger, metadata, sourceName, workerID, workerMetadata, delegate, resourceConfig, tags, params, _, _ := fakeFactory.GetArgsForCall(1)
				Expect(logger).NotTo(BeNil())
				Expect(metadata).To(Equal(expectedMetadata))
				Expect(workerMetadata).To(Equal(worker.Metadata{
					ResourceName: "",
					Type:         db.ContainerTypeGet,
					StepName:     "some-get",
					PipelineName: "some-pipeline",
					Attempts:     []int{3},
				}))
				Expect(workerID).To(Equal(worker.Identifier{
					BuildID: 42,
					PlanID:  getPlan.ID,
				}))

				Expect(tags).To(BeEmpty())
				Expect(delegate).To(Equal(fakeInputDelegate))

				Expect(sourceName).To(Equal(exec.SourceName("some-get")))
				Expect(resourceConfig.Name).To(Equal("some-input-resource"))
				Expect(resourceConfig.Type).To(Equal("get"))
				Expect(resourceConfig.Source).To(Equal(atc.Source{"some": "source"}))
				Expect(params).To(Equal(atc.Params{"some": "params"}))
			})

			It("constructs nested retries correctly", func() {
				Expect(*retryPlanTwo.Retry).To(HaveLen(2))
			})

			It("constructs nested steps correctly", func() {
				logger, sourceName, workerID, workerMetadata, delegate, privileged, tags, configSource, _, _, _ := fakeFactory.TaskArgsForCall(0)
				Expect(logger).NotTo(BeNil())
				Expect(sourceName).To(Equal(exec.SourceName("some-task")))
				Expect(workerMetadata).To(Equal(worker.Metadata{
					ResourceName: "",
					Type:         db.ContainerTypeTask,
					StepName:     "some-task",
					PipelineName: "some-pipeline",
					Attempts:     []int{2, 1},
				}))
				Expect(workerID).To(Equal(worker.Identifier{
					BuildID: 42,
					PlanID:  taskPlan.ID,
				}))

				Expect(delegate).To(Equal(fakeExecutionDelegate))
				Expect(privileged).To(Equal(exec.Privileged(false)))
				Expect(tags).To(Equal(atc.Tags{"some", "task", "tags"}))
				Expect(configSource).To(Equal(exec.ValidatingConfigSource{exec.FileConfigSource{"some-config-path"}}))

				logger, sourceName, workerID, workerMetadata, delegate, privileged, tags, configSource, _, _, _ = fakeFactory.TaskArgsForCall(1)
				Expect(logger).NotTo(BeNil())
				Expect(sourceName).To(Equal(exec.SourceName("some-task")))
				Expect(workerMetadata).To(Equal(worker.Metadata{
					ResourceName: "",
					Type:         db.ContainerTypeTask,
					StepName:     "some-task",
					PipelineName: "some-pipeline",
					Attempts:     []int{2, 2},
				}))
				Expect(workerID).To(Equal(worker.Identifier{
					BuildID: 42,
					PlanID:  taskPlan.ID,
				}))

				Expect(delegate).To(Equal(fakeExecutionDelegate))
				Expect(privileged).To(Equal(exec.Privileged(false)))
				Expect(tags).To(Equal(atc.Tags{"some", "task", "tags"}))
				Expect(configSource).To(Equal(exec.ValidatingConfigSource{exec.FileConfigSource{"some-config-path"}}))
			})
		})

		Context("with a plan where conditional steps are inside retries", func() {
			var (
				retryPlan     atc.Plan
				onSuccessPlan atc.Plan
				onFailurePlan atc.Plan
				ensurePlan    atc.Plan
				leafPlan      atc.Plan
				err           error
			)
			BeforeEach(func() {
				leafPlan = planFactory.NewPlan(atc.TaskPlan{
					Name:       "some-task",
					Privileged: false,
					Tags:       atc.Tags{"some", "task", "tags"},
					Pipeline:   "some-pipeline",
					ConfigPath: "some-config-path",
				})

				onSuccessPlan = planFactory.NewPlan(atc.OnSuccessPlan{
					Step: leafPlan,
					Next: leafPlan,
				})

				onFailurePlan = planFactory.NewPlan(atc.OnFailurePlan{
					Step: onSuccessPlan,
					Next: leafPlan,
				})

				ensurePlan = planFactory.NewPlan(atc.EnsurePlan{
					Step: onFailurePlan,
					Next: leafPlan,
				})

				retryPlan = planFactory.NewPlan(atc.RetryPlan{
					ensurePlan,
				})

				build, err = execEngine.CreateBuild(logger, buildModel, retryPlan)
				Expect(err).NotTo(HaveOccurred())
				build.Resume(logger)
				Expect(fakeFactory.TaskCallCount()).To(Equal(4))
			})

			It("constructs nested steps correctly", func() {
				_, _, _, workerMetadata, _, _, _, _, _, _, _ := fakeFactory.TaskArgsForCall(0)
				Expect(workerMetadata.Attempts).To(Equal([]int{1}))
				_, _, _, workerMetadata, _, _, _, _, _, _, _ = fakeFactory.TaskArgsForCall(1)
				Expect(workerMetadata.Attempts).To(Equal([]int{1}))
				_, _, _, workerMetadata, _, _, _, _, _, _, _ = fakeFactory.TaskArgsForCall(2)
				Expect(workerMetadata.Attempts).To(Equal([]int{1}))
				_, _, _, workerMetadata, _, _, _, _, _, _, _ = fakeFactory.TaskArgsForCall(3)
				Expect(workerMetadata.Attempts).To(Equal([]int{1}))
			})
		})

		Context("with a basic plan", func() {
			var plan atc.Plan
			Context("that contains inputs", func() {
				BeforeEach(func() {
					getPlan := atc.GetPlan{
						Name:     "some-input",
						Resource: "some-input-resource",
						Type:     "get",
						Tags:     []string{"some", "get", "tags"},
						Version:  atc.Version{"some": "version"},
						Source:   atc.Source{"some": "source"},
						Params:   atc.Params{"some": "params"},
						Pipeline: "some-pipeline",
					}

					plan = planFactory.NewPlan(getPlan)
				})

				It("constructs inputs correctly", func() {
					var err error
					build, err := execEngine.CreateBuild(logger, buildModel, plan)
					Expect(err).NotTo(HaveOccurred())

					build.Resume(logger)
					Expect(fakeFactory.GetCallCount()).To(Equal(1))

					logger, metadata, sourceName, workerID, workerMetadata, delegate, resourceConfig, tags, params, version, _ := fakeFactory.GetArgsForCall(0)
					Expect(logger).NotTo(BeNil())
					Expect(metadata).To(Equal(expectedMetadata))
					Expect(workerMetadata).To(Equal(worker.Metadata{
						ResourceName: "",
						Type:         db.ContainerTypeGet,
						StepName:     "some-input",
						PipelineName: "some-pipeline",
					}))
					Expect(sourceName).To(Equal(exec.SourceName("some-input")))
					Expect(workerID).To(Equal(worker.Identifier{
						BuildID: 42,
						PlanID:  plan.ID,
					}))

					Expect(tags).To(ConsistOf("some", "get", "tags"))
					Expect(resourceConfig.Name).To(Equal("some-input-resource"))
					Expect(resourceConfig.Type).To(Equal("get"))
					Expect(resourceConfig.Source).To(Equal(atc.Source{"some": "source"}))
					Expect(params).To(Equal(atc.Params{"some": "params"}))
					Expect(version).To(Equal(atc.Version{"some": "version"}))

					Expect(delegate).To(Equal(fakeInputDelegate))

					_, _, planID := fakeDelegate.InputDelegateArgsForCall(0)
					Expect(planID).To(Equal(event.OriginID(plan.ID)))
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
				var (
					inputMapping  map[string]string
					outputMapping map[string]string
					taskPlan      atc.TaskPlan
				)

				BeforeEach(func() {
					inputMapping = map[string]string{"foo": "bar"}
					outputMapping = map[string]string{"baz": "qux"}

					taskPlan = atc.TaskPlan{
						Name:          "some-task",
						ConfigPath:    "some-input/build.yml",
						Pipeline:      "some-pipeline",
						InputMapping:  inputMapping,
						OutputMapping: outputMapping,
					}
				})

				JustBeforeEach(func() {
					plan = planFactory.NewPlan(taskPlan)
				})

				It("constructs tasks correctly", func() {
					var err error
					build, err = execEngine.CreateBuild(logger, buildModel, plan)
					Expect(err).NotTo(HaveOccurred())

					build.Resume(logger)
					Expect(fakeFactory.TaskCallCount()).To(Equal(1))

					logger, sourceName, workerID, workerMetadata, delegate, privileged, tags, configSource, _, actualInputMapping, actualOutputMapping := fakeFactory.TaskArgsForCall(0)
					Expect(logger).NotTo(BeNil())
					Expect(sourceName).To(Equal(exec.SourceName("some-task")))
					Expect(workerMetadata).To(Equal(worker.Metadata{
						ResourceName: "",
						Type:         db.ContainerTypeTask,
						StepName:     "some-task",
						PipelineName: "some-pipeline",
					}))
					Expect(workerID).To(Equal(worker.Identifier{
						BuildID: 42,
						PlanID:  plan.ID,
					}))

					Expect(privileged).To(Equal(exec.Privileged(false)))
					Expect(tags).To(BeEmpty())
					Expect(configSource).NotTo(BeNil())

					Expect(delegate).To(Equal(fakeExecutionDelegate))

					_, _, planID := fakeDelegate.ExecutionDelegateArgsForCall(0)
					Expect(planID).To(Equal(event.OriginID(plan.ID)))

					Expect(actualInputMapping).To(Equal(inputMapping))
					Expect(actualOutputMapping).To(Equal(outputMapping))
				})

				Context("when the plan contains params and config path", func() {
					BeforeEach(func() {
						taskPlan.Params = map[string]interface{}{
							"task-param": "task-param-value",
						}
					})

					It("creates the task with a MergedConfigSource wrapped in a ValidatingConfigSource", func() {
						var err error
						build, err = execEngine.CreateBuild(logger, buildModel, plan)
						Expect(err).NotTo(HaveOccurred())

						build.Resume(logger)
						Expect(fakeFactory.TaskCallCount()).To(Equal(1))

						_, _, _, _, _, _, _, configSource, _, _, _ := fakeFactory.TaskArgsForCall(0)
						vcs, ok := configSource.(exec.ValidatingConfigSource)
						Expect(ok).To(BeTrue())
						_, ok = vcs.ConfigSource.(exec.MergedConfigSource)
						Expect(ok).To(BeTrue())
					})
				})

				Context("when the plan contains config and config path", func() {
					BeforeEach(func() {
						taskPlan.Config = &atc.TaskConfig{
							Params: map[string]string{
								"task-param": "task-param-value",
							},
						}
					})

					It("creates the task with a MergedConfigSource wrapped in a ValidatingConfigSource", func() {
						var err error
						build, err = execEngine.CreateBuild(logger, buildModel, plan)
						Expect(err).NotTo(HaveOccurred())

						build.Resume(logger)
						Expect(fakeFactory.TaskCallCount()).To(Equal(1))

						_, _, _, _, _, _, _, configSource, _, _, _ := fakeFactory.TaskArgsForCall(0)
						vcs, ok := configSource.(exec.ValidatingConfigSource)
						Expect(ok).To(BeTrue())
						_, ok = vcs.ConfigSource.(exec.MergedConfigSource)
						Expect(ok).To(BeTrue())
					})
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
			})

			Context("that contains outputs", func() {
				var (
					plan             atc.Plan
					putPlan          atc.Plan
					dependentGetPlan atc.Plan
				)

				BeforeEach(func() {
					putPlan = planFactory.NewPlan(atc.PutPlan{
						Name:     "some-put",
						Resource: "some-output-resource",
						Tags:     []string{"some", "putget", "tags"},
						Type:     "put",
						Source:   atc.Source{"some": "source"},
						Params:   atc.Params{"some": "params"},
						Pipeline: "some-pipeline",
					})
					dependentGetPlan = planFactory.NewPlan(atc.DependentGetPlan{
						Name:     "some-get",
						Resource: "some-input-resource",
						Tags:     []string{"some", "putget", "tags"},
						Type:     "get",
						Source:   atc.Source{"some": "source"},
						Params:   atc.Params{"another": "params"},
						Pipeline: "some-pipeline",
					})

					plan = planFactory.NewPlan(atc.OnSuccessPlan{
						Step: putPlan,
						Next: dependentGetPlan,
					})
				})

				It("constructs the put correctly", func() {
					var err error
					build, err = execEngine.CreateBuild(logger, buildModel, plan)
					Expect(err).NotTo(HaveOccurred())

					build.Resume(logger)
					Expect(fakeFactory.PutCallCount()).To(Equal(1))

					logger, metadata, workerID, workerMetadata, delegate, resourceConfig, tags, params, _ := fakeFactory.PutArgsForCall(0)
					Expect(logger).NotTo(BeNil())
					Expect(metadata).To(Equal(expectedMetadata))
					Expect(workerMetadata).To(Equal(worker.Metadata{
						ResourceName: "",
						Type:         db.ContainerTypePut,
						StepName:     "some-put",
						PipelineName: "some-pipeline",
					}))
					Expect(workerID).To(Equal(worker.Identifier{
						BuildID: 42,
						PlanID:  putPlan.ID,
					}))

					Expect(resourceConfig.Name).To(Equal("some-output-resource"))
					Expect(resourceConfig.Type).To(Equal("put"))
					Expect(resourceConfig.Source).To(Equal(atc.Source{"some": "source"}))
					Expect(tags).To(ConsistOf("some", "putget", "tags"))
					Expect(params).To(Equal(atc.Params{"some": "params"}))

					Expect(delegate).To(Equal(fakeOutputDelegate))

					_, _, planID := fakeDelegate.OutputDelegateArgsForCall(0)
					Expect(planID).To(Equal(event.OriginID(putPlan.ID)))
				})

				It("constructs the dependent get correctly", func() {
					var err error
					build, err = execEngine.CreateBuild(logger, buildModel, plan)
					Expect(err).NotTo(HaveOccurred())

					build.Resume(logger)
					Expect(fakeFactory.DependentGetCallCount()).To(Equal(1))

					logger, metadata, sourceName, workerID, workerMetadata, delegate, resourceConfig, tags, params, _ := fakeFactory.DependentGetArgsForCall(0)
					Expect(logger).NotTo(BeNil())
					Expect(metadata).To(Equal(expectedMetadata))
					Expect(workerMetadata).To(Equal(worker.Metadata{
						ResourceName: "",
						Type:         db.ContainerTypeGet,
						StepName:     "some-get",
						PipelineName: "some-pipeline",
					}))
					Expect(workerID).To(Equal(worker.Identifier{
						BuildID: 42,
						PlanID:  dependentGetPlan.ID,
					}))

					Expect(tags).To(ConsistOf("some", "putget", "tags"))
					Expect(sourceName).To(Equal(exec.SourceName("some-get")))
					Expect(resourceConfig.Name).To(Equal("some-input-resource"))
					Expect(resourceConfig.Type).To(Equal("get"))
					Expect(resourceConfig.Source).To(Equal(atc.Source{"some": "source"}))
					Expect(params).To(Equal(atc.Params{"another": "params"}))

					Expect(delegate).To(Equal(fakeInputDelegate))

					_, _, planID := fakeDelegate.InputDelegateArgsForCall(0)
					Expect(planID).To(Equal(event.OriginID(dependentGetPlan.ID)))
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

	Describe("PublicPlan", func() {
		var build engine.Build
		var logger lager.Logger

		var plan atc.Plan

		var publicPlan atc.PublicBuildPlan
		var planFound bool
		var publicPlanErr error

		BeforeEach(func() {
			logger = lagertest.NewTestLogger("test")

			planFactory := atc.NewPlanFactory(123)

			plan = planFactory.NewPlan(atc.OnSuccessPlan{
				Step: planFactory.NewPlan(atc.PutPlan{
					Name:     "some-put",
					Resource: "some-output-resource",
					Tags:     []string{"some", "putget", "tags"},
					Type:     "some-type",
					Source:   atc.Source{"some": "source"},
					Params:   atc.Params{"some": "params"},
					Pipeline: "some-pipeline",
				}),
				Next: planFactory.NewPlan(atc.DependentGetPlan{
					Name:     "some-put",
					Resource: "some-output-resource",
					Tags:     []string{"some", "putget", "tags"},
					Type:     "some-type",
					Source:   atc.Source{"some": "source"},
					Params:   atc.Params{"another": "params"},
				}),
			})

			var err error
			build, err = execEngine.CreateBuild(logger, db.Build{ID: 123}, plan)
			Expect(err).ToNot(HaveOccurred())
		})

		JustBeforeEach(func() {
			publicPlan, planFound, publicPlanErr = build.PublicPlan(logger)
		})

		It("returns the plan successfully", func() {
			Expect(publicPlanErr).ToNot(HaveOccurred())
			Expect(planFound).To(BeTrue())
		})

		It("has the engine name as the schema", func() {
			Expect(publicPlan.Schema).To(Equal("exec.v2"))
		})

		It("cleans out sensitive/irrelevant information from the original plan", func() {
			Expect(publicPlan.Plan).To(Equal(plan.Public()))
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
