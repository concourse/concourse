package engine_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/db/dbfakes"
	"github.com/concourse/atc/engine"
	"github.com/concourse/atc/engine/enginefakes"
	"github.com/concourse/atc/event"
	"github.com/concourse/atc/exec"
	"github.com/concourse/atc/exec/execfakes"
	"github.com/concourse/atc/worker"
	"github.com/pivotal-golang/lager"
	"github.com/pivotal-golang/lager/lagertest"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("ExecEngine", func() {
	var (
		fakeFactory         *execfakes.FakeFactory
		fakeTeamDB          *dbfakes.FakeTeamDB
		fakeDelegateFactory *enginefakes.FakeBuildDelegateFactory
		logger              *lagertest.TestLogger

		execEngine engine.Engine
	)

	BeforeEach(func() {
		fakeFactory = new(execfakes.FakeFactory)
		fakeDelegateFactory = new(enginefakes.FakeBuildDelegateFactory)
		logger = lagertest.NewTestLogger("test")

		fakeTeamDBFactory := new(dbfakes.FakeTeamDBFactory)
		fakeTeamDB = new(dbfakes.FakeTeamDB)
		fakeTeamDBFactory.GetTeamDBReturns(fakeTeamDB)
		execEngine = engine.NewExecEngine(
			fakeFactory,
			fakeDelegateFactory,
			fakeTeamDBFactory,
			"http://example.com",
		)
	})

	Describe("Resume", func() {
		var (
			fakeDelegate          *enginefakes.FakeBuildDelegate
			fakeInputDelegate     *execfakes.FakeGetDelegate
			fakeExecutionDelegate *execfakes.FakeTaskDelegate
			fakeOutputDelegate    *execfakes.FakePutDelegate

			dbBuild          *dbfakes.FakeBuild
			expectedMetadata engine.StepMetadata

			outputPlan atc.Plan

			build engine.Build

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
			planFactory = atc.NewPlanFactory(123)

			dbBuild = new(dbfakes.FakeBuild)
			dbBuild.IDReturns(42)
			dbBuild.NameReturns("21")
			dbBuild.JobNameReturns("some-job")
			dbBuild.PipelineNameReturns("some-pipeline")
			dbBuild.TeamNameReturns("some-team")
			dbBuild.TeamIDReturns(17)

			expectedMetadata = engine.StepMetadata{
				BuildID:      42,
				BuildName:    "21",
				JobName:      "some-job",
				PipelineName: "some-pipeline",
				ExternalURL:  "http://example.com",
			}

			fakeDelegate = new(enginefakes.FakeBuildDelegate)
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
					Name:       "some-put",
					Resource:   "some-output-resource",
					Type:       "put",
					Source:     atc.Source{"some": "source"},
					Params:     atc.Params{"some": "params"},
					PipelineID: 57,
				})
				dependentGetPlan = planFactory.NewPlan(atc.DependentGetPlan{
					Name:       "some-get",
					Resource:   "some-input-resource",
					Type:       "get",
					Source:     atc.Source{"some": "source"},
					Params:     atc.Params{"another": "params"},
					PipelineID: 57,
				})

				otherPutPlan = planFactory.NewPlan(atc.PutPlan{
					Name:       "some-put-2",
					Resource:   "some-output-resource-2",
					Type:       "put",
					Source:     atc.Source{"some": "source-2"},
					Params:     atc.Params{"some": "params-2"},
					PipelineID: 57,
				})
				otherDependentGetPlan = planFactory.NewPlan(atc.DependentGetPlan{
					Name:       "some-get-2",
					Resource:   "some-input-resource-2",
					Type:       "get",
					Source:     atc.Source{"some": "source-2"},
					Params:     atc.Params{"another": "params-2"},
					PipelineID: 57,
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

			Context("when one-off build", func() {
				BeforeEach(func() {
					dbBuild.IsOneOffReturns(true)
				})

				It("constructs the put with finite container TTLs", func() {
					var err error
					build, err = execEngine.CreateBuild(logger, dbBuild, outputPlan)
					Expect(err).NotTo(HaveOccurred())

					build.Resume(logger)
					Expect(fakeFactory.PutCallCount()).To(Equal(2))

					_, _, _, _, _, _, _, _, _, _, containerSuccessTTL, containerFailureTTL := fakeFactory.PutArgsForCall(0)
					Expect(containerSuccessTTL).To(Equal(5 * time.Minute))
					Expect(containerFailureTTL).To(Equal(5 * time.Minute))
				})

				It("constructs the dependent get with finite container TTLs", func() {
					var err error
					build, err = execEngine.CreateBuild(logger, dbBuild, outputPlan)
					Expect(err).NotTo(HaveOccurred())

					build.Resume(logger)
					Expect(fakeFactory.PutCallCount()).To(Equal(2))

					_, _, _, _, _, _, _, _, _, _, containerSuccessTTL, containerFailureTTL := fakeFactory.PutArgsForCall(0)
					Expect(containerSuccessTTL).To(Equal(5 * time.Minute))
					Expect(containerFailureTTL).To(Equal(5 * time.Minute))
				})
			})

			Context("when JobID is nonzero (job build)", func() {
				BeforeEach(func() {
					dbBuild.IsOneOffReturns(false)
				})

				It("constructs the put with infinite container TTLs", func() {
					var err error
					build, err = execEngine.CreateBuild(logger, dbBuild, outputPlan)
					Expect(err).NotTo(HaveOccurred())

					build.Resume(logger)
					Expect(fakeFactory.PutCallCount()).To(Equal(2))

					_, _, _, _, _, _, _, _, _, _, containerSuccessTTL, containerFailureTTL := fakeFactory.PutArgsForCall(0)
					Expect(containerSuccessTTL).To(Equal(5 * time.Minute))
					Expect(containerFailureTTL).To(Equal(5 * time.Minute))
				})

				It("constructs the dependent get with infinite container TTLs", func() {
					var err error
					build, err = execEngine.CreateBuild(logger, dbBuild, outputPlan)
					Expect(err).NotTo(HaveOccurred())

					build.Resume(logger)
					Expect(fakeFactory.DependentGetCallCount()).To(Equal(2))

					_, _, _, _, _, _, _, _, _, _, _, containerSuccessTTL, containerFailureTTL := fakeFactory.DependentGetArgsForCall(0)
					Expect(containerSuccessTTL).To(Equal(5 * time.Minute))
					Expect(containerFailureTTL).To(Equal(5 * time.Minute))
				})
			})

			Context("constructing outputs", func() {
				It("constructs the put correctly", func() {
					var err error
					build, err = execEngine.CreateBuild(logger, dbBuild, outputPlan)
					Expect(err).NotTo(HaveOccurred())

					build.Resume(logger)
					Expect(fakeFactory.PutCallCount()).To(Equal(2))

					logger, metadata, workerID, workerMetadata, delegate, resourceConfig, tags, teamName, params, _, _, _ := fakeFactory.PutArgsForCall(0)
					Expect(logger).NotTo(BeNil())
					Expect(metadata).To(Equal(expectedMetadata))
					Expect(workerMetadata).To(Equal(worker.Metadata{
						ResourceName: "",
						Type:         db.ContainerTypePut,
						StepName:     "some-put",
						PipelineID:   57,
						TeamID:       17,
					}))
					Expect(workerID).To(Equal(worker.Identifier{
						BuildID: 42,
						PlanID:  putPlan.ID,
					}))

					Expect(tags).To(BeEmpty())
					Expect(teamName).To(Equal("some-team"))
					Expect(delegate).To(Equal(fakeOutputDelegate))
					Expect(resourceConfig.Name).To(Equal("some-output-resource"))
					Expect(resourceConfig.Type).To(Equal("put"))
					Expect(resourceConfig.Source).To(Equal(atc.Source{"some": "source"}))
					Expect(params).To(Equal(atc.Params{"some": "params"}))

					logger, metadata, workerID, workerMetadata, delegate, resourceConfig, tags, teamName, params, _, _, _ = fakeFactory.PutArgsForCall(1)
					Expect(logger).NotTo(BeNil())
					Expect(metadata).To(Equal(expectedMetadata))
					Expect(workerMetadata).To(Equal(worker.Metadata{
						ResourceName: "",
						Type:         db.ContainerTypePut,
						StepName:     "some-put-2",
						PipelineID:   57,
						TeamID:       17,
					}))
					Expect(workerID).To(Equal(worker.Identifier{
						BuildID: 42,
						PlanID:  otherPutPlan.ID,
					}))

					Expect(tags).To(BeEmpty())
					Expect(teamName).To(Equal("some-team"))
					Expect(delegate).To(Equal(fakeOutputDelegate))
					Expect(resourceConfig.Name).To(Equal("some-output-resource-2"))
					Expect(resourceConfig.Type).To(Equal("put"))
					Expect(resourceConfig.Source).To(Equal(atc.Source{"some": "source-2"}))
					Expect(params).To(Equal(atc.Params{"some": "params-2"}))
				})

				It("constructs the dependent get correctly", func() {
					var err error
					build, err = execEngine.CreateBuild(logger, dbBuild, outputPlan)
					Expect(err).NotTo(HaveOccurred())

					build.Resume(logger)
					Expect(fakeFactory.DependentGetCallCount()).To(Equal(2))

					logger, metadata, sourceName, workerID, workerMetadata, delegate, resourceConfig, tags, teamName, params, _, _, _ := fakeFactory.DependentGetArgsForCall(0)
					Expect(logger).NotTo(BeNil())
					Expect(metadata).To(Equal(expectedMetadata))
					Expect(workerMetadata).To(Equal(worker.Metadata{
						ResourceName: "",
						Type:         db.ContainerTypeGet,
						StepName:     "some-get",
						PipelineID:   57,
						TeamID:       17,
					}))
					Expect(workerID).To(Equal(worker.Identifier{
						BuildID: 42,
						PlanID:  dependentGetPlan.ID,
					}))

					Expect(tags).To(BeEmpty())
					Expect(teamName).To(Equal("some-team"))
					Expect(delegate).To(Equal(fakeInputDelegate))
					_, plan, planID := fakeDelegate.InputDelegateArgsForCall(0)
					Expect(plan).To(Equal((*outputPlan.Aggregate)[0].OnSuccess.Next.DependentGet.GetPlan()))
					Expect(planID).NotTo(BeNil())

					Expect(sourceName).To(Equal(exec.SourceName("some-get")))
					Expect(resourceConfig.Name).To(Equal("some-input-resource"))
					Expect(resourceConfig.Type).To(Equal("get"))
					Expect(resourceConfig.Source).To(Equal(atc.Source{"some": "source"}))
					Expect(params).To(Equal(atc.Params{"another": "params"}))

					logger, metadata, sourceName, workerID, workerMetadata, delegate, resourceConfig, tags, teamName, params, _, _, _ = fakeFactory.DependentGetArgsForCall(1)
					Expect(logger).NotTo(BeNil())
					Expect(metadata).To(Equal(expectedMetadata))
					Expect(workerMetadata).To(Equal(worker.Metadata{
						ResourceName: "",
						Type:         db.ContainerTypeGet,
						StepName:     "some-get-2",
						PipelineID:   57,
						TeamID:       17,
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
					Name:       "some-get",
					Resource:   "some-input-resource",
					Type:       "get",
					Source:     atc.Source{"some": "source"},
					Params:     atc.Params{"some": "params"},
					PipelineID: 57,
				})

				taskPlan = planFactory.NewPlan(atc.TaskPlan{
					Name:       "some-task",
					Privileged: false,
					Tags:       atc.Tags{"some", "task", "tags"},
					PipelineID: 57,
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

				build, err = execEngine.CreateBuild(logger, dbBuild, retryPlan)
				Expect(err).NotTo(HaveOccurred())
				build.Resume(logger)
				Expect(fakeFactory.GetCallCount()).To(Equal(2))
				Expect(fakeFactory.TaskCallCount()).To(Equal(2))
			})

			It("constructs the retry correctly", func() {
				Expect(*retryPlan.Retry).To(HaveLen(3))
			})

			It("constructs the first get correctly", func() {
				logger, metadata, sourceName, workerID, workerMetadata, delegate, resourceConfig, tags, teamName, params, _, _, _, _ := fakeFactory.GetArgsForCall(0)
				Expect(logger).NotTo(BeNil())
				Expect(metadata).To(Equal(expectedMetadata))
				Expect(workerMetadata).To(Equal(worker.Metadata{
					ResourceName: "",
					Type:         db.ContainerTypeGet,
					StepName:     "some-get",
					PipelineID:   57,
					Attempts:     []int{1},
					TeamID:       17,
				}))
				Expect(workerID).To(Equal(worker.Identifier{
					BuildID: 42,
					PlanID:  getPlan.ID,
				}))

				Expect(tags).To(BeEmpty())
				Expect(teamName).To(Equal("some-team"))
				Expect(delegate).To(Equal(fakeInputDelegate))

				Expect(sourceName).To(Equal(exec.SourceName("some-get")))
				Expect(resourceConfig.Name).To(Equal("some-input-resource"))
				Expect(resourceConfig.Type).To(Equal("get"))
				Expect(resourceConfig.Source).To(Equal(atc.Source{"some": "source"}))
				Expect(params).To(Equal(atc.Params{"some": "params"}))
			})

			It("constructs the second get correctly", func() {
				logger, metadata, sourceName, workerID, workerMetadata, delegate, resourceConfig, tags, teamName, params, _, _, _, _ := fakeFactory.GetArgsForCall(1)
				Expect(logger).NotTo(BeNil())
				Expect(metadata).To(Equal(expectedMetadata))
				Expect(workerMetadata).To(Equal(worker.Metadata{
					ResourceName: "",
					Type:         db.ContainerTypeGet,
					StepName:     "some-get",
					PipelineID:   57,
					Attempts:     []int{3},
					TeamID:       17,
				}))
				Expect(workerID).To(Equal(worker.Identifier{
					BuildID: 42,
					PlanID:  getPlan.ID,
				}))

				Expect(tags).To(BeEmpty())
				Expect(teamName).To(Equal("some-team"))
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
				logger, sourceName, workerID, workerMetadata, delegate, privileged, tags, teamName, configSource, _, _, _, _, _, _, _ := fakeFactory.TaskArgsForCall(0)
				Expect(logger).NotTo(BeNil())
				Expect(sourceName).To(Equal(exec.SourceName("some-task")))
				Expect(workerMetadata).To(Equal(worker.Metadata{
					ResourceName: "",
					Type:         db.ContainerTypeTask,
					StepName:     "some-task",
					PipelineID:   57,
					Attempts:     []int{2, 1},
					TeamID:       17,
				}))
				Expect(workerID).To(Equal(worker.Identifier{
					BuildID: 42,
					PlanID:  taskPlan.ID,
				}))

				Expect(delegate).To(Equal(fakeExecutionDelegate))
				Expect(privileged).To(Equal(exec.Privileged(false)))
				Expect(tags).To(Equal(atc.Tags{"some", "task", "tags"}))
				Expect(teamName).To(Equal("some-team"))
				Expect(configSource).To(Equal(exec.ValidatingConfigSource{exec.FileConfigSource{"some-config-path"}}))

				logger, sourceName, workerID, workerMetadata, delegate, privileged, tags, teamName, configSource, _, _, _, _, _, _, _ = fakeFactory.TaskArgsForCall(1)
				Expect(logger).NotTo(BeNil())
				Expect(sourceName).To(Equal(exec.SourceName("some-task")))
				Expect(workerMetadata).To(Equal(worker.Metadata{
					ResourceName: "",
					Type:         db.ContainerTypeTask,
					StepName:     "some-task",
					PipelineID:   57,
					Attempts:     []int{2, 2},
					TeamID:       17,
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

				build, err = execEngine.CreateBuild(logger, dbBuild, retryPlan)
				Expect(err).NotTo(HaveOccurred())
				build.Resume(logger)
				Expect(fakeFactory.TaskCallCount()).To(Equal(4))
			})

			It("constructs nested steps correctly", func() {
				_, _, _, workerMetadata, _, _, _, _, _, _, _, _, _, _, _, _ := fakeFactory.TaskArgsForCall(0)
				Expect(workerMetadata.Attempts).To(Equal([]int{1}))
				_, _, _, workerMetadata, _, _, _, _, _, _, _, _, _, _, _, _ = fakeFactory.TaskArgsForCall(1)
				Expect(workerMetadata.Attempts).To(Equal([]int{1}))
				_, _, _, workerMetadata, _, _, _, _, _, _, _, _, _, _, _, _ = fakeFactory.TaskArgsForCall(2)
				Expect(workerMetadata.Attempts).To(Equal([]int{1}))
				_, _, _, workerMetadata, _, _, _, _, _, _, _, _, _, _, _, _ = fakeFactory.TaskArgsForCall(3)
				Expect(workerMetadata.Attempts).To(Equal([]int{1}))
			})
		})

		Context("with a basic plan", func() {
			var plan atc.Plan
			Context("that contains inputs", func() {
				BeforeEach(func() {
					getPlan := atc.GetPlan{
						Name:       "some-input",
						Resource:   "some-input-resource",
						Type:       "get",
						Tags:       []string{"some", "get", "tags"},
						Version:    atc.Version{"some": "version"},
						Source:     atc.Source{"some": "source"},
						Params:     atc.Params{"some": "params"},
						PipelineID: 57,
					}

					plan = planFactory.NewPlan(getPlan)
				})

				Context("when one-off build", func() {
					BeforeEach(func() {
						dbBuild.IsOneOffReturns(true)
					})

					It("constructs the get with container TTLs", func() {
						var err error
						build, err = execEngine.CreateBuild(logger, dbBuild, plan)
						Expect(err).NotTo(HaveOccurred())

						build.Resume(logger)
						Expect(fakeFactory.GetCallCount()).To(Equal(1))

						_, _, _, _, _, _, _, _, _, _, _, _, containerSuccessTTL, containerFailureTTL := fakeFactory.GetArgsForCall(0)
						Expect(containerSuccessTTL).To(Equal(5 * time.Minute))
						Expect(containerFailureTTL).To(Equal(5 * time.Minute))
					})
				})

				Context("when build is not one-off", func() {
					BeforeEach(func() {
						dbBuild.IsOneOffReturns(false)
					})

					It("constructs the get with container TTLs", func() {
						var err error
						build, err = execEngine.CreateBuild(logger, dbBuild, plan)
						Expect(err).NotTo(HaveOccurred())

						build.Resume(logger)
						Expect(fakeFactory.GetCallCount()).To(Equal(1))

						_, _, _, _, _, _, _, _, _, _, _, _, containerSuccessTTL, containerFailureTTL := fakeFactory.GetArgsForCall(0)
						Expect(containerSuccessTTL).To(Equal(5 * time.Minute))
						Expect(containerFailureTTL).To(Equal(5 * time.Minute))
					})
				})

				It("constructs inputs correctly", func() {
					var err error
					build, err := execEngine.CreateBuild(logger, dbBuild, plan)
					Expect(err).NotTo(HaveOccurred())

					build.Resume(logger)
					Expect(fakeFactory.GetCallCount()).To(Equal(1))

					logger, metadata, sourceName, workerID, workerMetadata, delegate, resourceConfig, tags, teamName, params, version, _, _, _ := fakeFactory.GetArgsForCall(0)
					Expect(logger).NotTo(BeNil())
					Expect(metadata).To(Equal(expectedMetadata))
					Expect(workerMetadata).To(Equal(worker.Metadata{
						ResourceName: "",
						Type:         db.ContainerTypeGet,
						StepName:     "some-input",
						PipelineID:   57,
						TeamID:       17,
					}))
					Expect(sourceName).To(Equal(exec.SourceName("some-input")))
					Expect(workerID).To(Equal(worker.Identifier{
						BuildID: 42,
						PlanID:  plan.ID,
					}))

					Expect(tags).To(ConsistOf("some", "get", "tags"))
					Expect(teamName).To(Equal("some-team"))
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
					build, err = execEngine.CreateBuild(logger, dbBuild, plan)
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
						PipelineID:    57,
						InputMapping:  inputMapping,
						OutputMapping: outputMapping,
					}
				})

				JustBeforeEach(func() {
					plan = planFactory.NewPlan(taskPlan)
				})

				Context("when one-off build", func() {
					BeforeEach(func() {
						dbBuild.IsOneOffReturns(true)
					})

					It("constructs the task with container TTLs", func() {
						var err error
						build, err = execEngine.CreateBuild(logger, dbBuild, plan)
						Expect(err).NotTo(HaveOccurred())

						build.Resume(logger)
						Expect(fakeFactory.TaskCallCount()).To(Equal(1))

						_, _, _, _, _, _, _, _, _, _, _, _, _, _, containerSuccessTTL, containerFailureTTL := fakeFactory.TaskArgsForCall(0)
						Expect(containerSuccessTTL).To(Equal(5 * time.Minute))
						Expect(containerFailureTTL).To(Equal(5 * time.Minute))
					})
				})

				Context("when build is not one-off", func() {
					BeforeEach(func() {
						dbBuild.IsOneOffReturns(false)
					})

					It("constructs the task with infinite container TTLs", func() {
						var err error
						build, err = execEngine.CreateBuild(logger, dbBuild, plan)
						Expect(err).NotTo(HaveOccurred())

						build.Resume(logger)
						Expect(fakeFactory.TaskCallCount()).To(Equal(1))

						_, _, _, _, _, _, _, _, _, _, _, _, _, _, containerSuccessTTL, containerFailureTTL := fakeFactory.TaskArgsForCall(0)
						Expect(containerSuccessTTL).To(Equal(5 * time.Minute))
						Expect(containerFailureTTL).To(Equal(5 * time.Minute))
					})

					It("constructs tasks correctly", func() {
						var err error
						build, err = execEngine.CreateBuild(logger, dbBuild, plan)
						Expect(err).NotTo(HaveOccurred())

						build.Resume(logger)
						Expect(fakeFactory.TaskCallCount()).To(Equal(1))

						logger, sourceName, workerID, workerMetadata, delegate, privileged, tags, teamName, configSource, _, actualInputMapping, actualOutputMapping, _, _, _, _ := fakeFactory.TaskArgsForCall(0)
						Expect(logger).NotTo(BeNil())
						Expect(sourceName).To(Equal(exec.SourceName("some-task")))
						Expect(workerMetadata).To(Equal(worker.Metadata{
							ResourceName: "",
							Type:         db.ContainerTypeTask,
							StepName:     "some-task",
							PipelineID:   57,
							TeamID:       17,
						}))
						Expect(workerID).To(Equal(worker.Identifier{
							BuildID: 42,
							PlanID:  plan.ID,
						}))

						Expect(privileged).To(Equal(exec.Privileged(false)))
						Expect(tags).To(BeEmpty())
						Expect(teamName).To(Equal("some-team"))
						Expect(configSource).NotTo(BeNil())

						Expect(delegate).To(Equal(fakeExecutionDelegate))

						_, _, planID := fakeDelegate.ExecutionDelegateArgsForCall(0)
						Expect(planID).To(Equal(event.OriginID(plan.ID)))

						Expect(actualInputMapping).To(Equal(inputMapping))
						Expect(actualOutputMapping).To(Equal(outputMapping))
					})

					Context("when the plan's image references the output of a previous step", func() {
						BeforeEach(func() {
							taskPlan.ImageArtifactName = "some-image-artifact-name"
						})

						It("constructs the task with the referenced image", func() {
							var err error
							build, err = execEngine.CreateBuild(logger, dbBuild, plan)
							Expect(err).NotTo(HaveOccurred())

							build.Resume(logger)
							Expect(fakeFactory.TaskCallCount()).To(Equal(1))

							_, _, _, _, _, _, _, _, _, _, _, _, actualImageArtifactName, _, _, _ := fakeFactory.TaskArgsForCall(0)
							Expect(actualImageArtifactName).To(Equal("some-image-artifact-name"))
						})
					})

					Context("when the plan contains params and config path", func() {
						BeforeEach(func() {
							taskPlan.Params = map[string]interface{}{
								"task-param": "task-param-value",
							}
						})

						It("creates the task with a MergedConfigSource wrapped in a ValidatingConfigSource", func() {
							var err error
							build, err = execEngine.CreateBuild(logger, dbBuild, plan)
							Expect(err).NotTo(HaveOccurred())

							build.Resume(logger)
							Expect(fakeFactory.TaskCallCount()).To(Equal(1))

							_, _, _, _, _, _, _, _, configSource, _, _, _, _, _, _, _ := fakeFactory.TaskArgsForCall(0)
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
							build, err = execEngine.CreateBuild(logger, dbBuild, plan)
							Expect(err).NotTo(HaveOccurred())

							build.Resume(logger)
							Expect(fakeFactory.TaskCallCount()).To(Equal(1))

							_, _, _, _, _, _, _, _, configSource, _, _, _, _, _, _, _ := fakeFactory.TaskArgsForCall(0)
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
						build, err = execEngine.CreateBuild(logger, dbBuild, plan)
						Expect(err).NotTo(HaveOccurred())
						build.Resume(logger)

						Expect(taskStep.ReleaseCallCount()).To(Equal(1))
					})
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
						Name:       "some-put",
						Resource:   "some-output-resource",
						Tags:       []string{"some", "putget", "tags"},
						Type:       "put",
						Source:     atc.Source{"some": "source"},
						Params:     atc.Params{"some": "params"},
						PipelineID: 57,
					})
					dependentGetPlan = planFactory.NewPlan(atc.DependentGetPlan{
						Name:       "some-get",
						Resource:   "some-input-resource",
						Tags:       []string{"some", "putget", "tags"},
						Type:       "get",
						Source:     atc.Source{"some": "source"},
						Params:     atc.Params{"another": "params"},
						PipelineID: 57,
					})

					plan = planFactory.NewPlan(atc.OnSuccessPlan{
						Step: putPlan,
						Next: dependentGetPlan,
					})
				})

				It("constructs the put correctly", func() {
					var err error
					build, err = execEngine.CreateBuild(logger, dbBuild, plan)
					Expect(err).NotTo(HaveOccurred())

					build.Resume(logger)
					Expect(fakeFactory.PutCallCount()).To(Equal(1))

					logger, metadata, workerID, workerMetadata, delegate, resourceConfig, tags, teamName, params, _, _, _ := fakeFactory.PutArgsForCall(0)
					Expect(logger).NotTo(BeNil())
					Expect(metadata).To(Equal(expectedMetadata))
					Expect(workerMetadata).To(Equal(worker.Metadata{
						ResourceName: "",
						Type:         db.ContainerTypePut,
						StepName:     "some-put",
						PipelineID:   57,
						TeamID:       17,
					}))
					Expect(workerID).To(Equal(worker.Identifier{
						BuildID: 42,
						PlanID:  putPlan.ID,
					}))

					Expect(resourceConfig.Name).To(Equal("some-output-resource"))
					Expect(resourceConfig.Type).To(Equal("put"))
					Expect(resourceConfig.Source).To(Equal(atc.Source{"some": "source"}))
					Expect(tags).To(ConsistOf("some", "putget", "tags"))
					Expect(teamName).To(Equal("some-team"))
					Expect(params).To(Equal(atc.Params{"some": "params"}))

					Expect(delegate).To(Equal(fakeOutputDelegate))

					_, _, planID := fakeDelegate.OutputDelegateArgsForCall(0)
					Expect(planID).To(Equal(event.OriginID(putPlan.ID)))
				})

				It("constructs the dependent get correctly", func() {
					var err error
					build, err = execEngine.CreateBuild(logger, dbBuild, plan)
					Expect(err).NotTo(HaveOccurred())

					build.Resume(logger)
					Expect(fakeFactory.DependentGetCallCount()).To(Equal(1))

					logger, metadata, sourceName, workerID, workerMetadata, delegate, resourceConfig, tags, teamName, params, _, _, _ := fakeFactory.DependentGetArgsForCall(0)
					Expect(logger).NotTo(BeNil())
					Expect(metadata).To(Equal(expectedMetadata))
					Expect(workerMetadata).To(Equal(worker.Metadata{
						ResourceName: "",
						Type:         db.ContainerTypeGet,
						StepName:     "some-get",
						PipelineID:   57,
						TeamID:       17,
					}))
					Expect(workerID).To(Equal(worker.Identifier{
						BuildID: 42,
						PlanID:  dependentGetPlan.ID,
					}))

					Expect(tags).To(ConsistOf("some", "putget", "tags"))
					Expect(teamName).To(Equal("some-team"))
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
					build, err = execEngine.CreateBuild(logger, dbBuild, plan)
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
			dbBuild := new(dbfakes.FakeBuild)
			build, err = execEngine.CreateBuild(logger, dbBuild, plan)
			Expect(err).ToNot(HaveOccurred())
		})

		JustBeforeEach(func() {
			publicPlan, publicPlanErr = build.PublicPlan(logger)
		})

		It("returns the plan successfully", func() {
			Expect(publicPlanErr).ToNot(HaveOccurred())
		})

		It("has the engine name as the schema", func() {
			Expect(publicPlan.Schema).To(Equal("exec.v2"))
		})

		It("cleans out sensitive/irrelevant information from the original plan", func() {
			Expect(publicPlan.Plan).To(Equal(plan.Public()))
		})
	})

	Describe("LookupBuild", func() {
		var dbBuild *dbfakes.FakeBuild
		BeforeEach(func() {
			dbBuild = new(dbfakes.FakeBuild)
			dbBuild.IDReturns(42)
			dbBuild.NameReturns("21")
			dbBuild.JobNameReturns("some-job")
			dbBuild.PipelineNameReturns("some-pipeline")
			dbBuild.TeamNameReturns("some-team")
		})

		Context("when the build has a get step", func() {
			var fakeInputDelegate *execfakes.FakeGetDelegate

			BeforeEach(func() {
				dbBuild.EngineMetadataReturns(`{
							"Plan": {
								"id": "47",
								"attempts": [1],
								"get": {
									"name": "some-get",
									"resource": "some-input-resource",
									"type": "get",
									"source": {"some": "source"},
									"params": {"some": "params"},
									"pipeline_id": 57
								}
							}
						}`,
				)

				fakeDelegate := new(enginefakes.FakeBuildDelegate)
				fakeDelegateFactory.DelegateReturns(fakeDelegate)

				inputStepFactory := new(execfakes.FakeStepFactory)
				inputStep := new(execfakes.FakeStep)
				inputStep.ResultStub = successResult(true)
				inputStepFactory.UsingReturns(inputStep)
				fakeFactory.GetReturns(inputStepFactory)
				fakeInputDelegate = new(execfakes.FakeGetDelegate)
				fakeDelegate.InputDelegateReturns(fakeInputDelegate)
			})

			It("constructs the get correctly", func() {
				foundBuild, err := execEngine.LookupBuild(logger, dbBuild)
				Expect(err).NotTo(HaveOccurred())

				foundBuild.Resume(logger)
				Expect(fakeFactory.GetCallCount()).To(Equal(1))
				logger, metadata, sourceName, workerID, workerMetadata, delegate, resourceConfig, tags, teamName, params, _, _, _, _ := fakeFactory.GetArgsForCall(0)
				Expect(logger).NotTo(BeNil())
				Expect(metadata).To(Equal(engine.StepMetadata{
					BuildID:      42,
					BuildName:    "21",
					JobName:      "some-job",
					PipelineName: "some-pipeline",
					ExternalURL:  "http://example.com",
				}))
				Expect(workerMetadata).To(Equal(worker.Metadata{
					ResourceName: "",
					Type:         db.ContainerTypeGet,
					StepName:     "some-get",
					PipelineID:   57,
					Attempts:     []int{1},
				}))
				Expect(workerID).To(Equal(worker.Identifier{
					BuildID: 42,
					PlanID:  "47",
				}))

				Expect(tags).To(BeEmpty())
				Expect(teamName).To(Equal("some-team"))
				Expect(delegate).To(Equal(fakeInputDelegate))

				Expect(sourceName).To(Equal(exec.SourceName("some-get")))
				Expect(resourceConfig.Name).To(Equal("some-input-resource"))
				Expect(resourceConfig.Type).To(Equal("get"))
				Expect(resourceConfig.Source).To(Equal(atc.Source{"some": "source"}))
				Expect(params).To(Equal(atc.Params{"some": "params"}))
			})

			Context("when one-off build", func() {
				BeforeEach(func() {
					dbBuild.IsOneOffReturns(true)
				})

				It("constructs the get with success and failure ttls", func() {
					foundBuild, err := execEngine.LookupBuild(logger, dbBuild)
					Expect(err).NotTo(HaveOccurred())

					foundBuild.Resume(logger)
					Expect(fakeFactory.GetCallCount()).To(Equal(1))

					_, _, _, _, _, _, _, _, _, _, _, _, containerSuccessTTL, containerFailureTTL := fakeFactory.GetArgsForCall(0)
					Expect(containerSuccessTTL).To(Equal(5 * time.Minute))
					Expect(containerFailureTTL).To(Equal(5 * time.Minute))
				})
			})

			Context("when build is not one-off", func() {
				BeforeEach(func() {
					dbBuild.IsOneOffReturns(false)
				})

				It("constructs the get with infinite container TTLs", func() {
					foundBuild, err := execEngine.LookupBuild(logger, dbBuild)
					Expect(err).NotTo(HaveOccurred())

					foundBuild.Resume(logger)
					Expect(fakeFactory.GetCallCount()).To(Equal(1))

					_, _, _, _, _, _, _, _, _, _, _, _, containerSuccessTTL, containerFailureTTL := fakeFactory.GetArgsForCall(0)
					Expect(containerSuccessTTL).To(Equal(5 * time.Minute))
					Expect(containerFailureTTL).To(Equal(5 * time.Minute))
				})
			})
		})

		Context("when pipeline name is specified and pipeline ID is not", func() {
			BeforeEach(func() {
				dbBuild.EngineMetadataReturns(`{
						"Plan": {
							"id": "1",
							"do": [
								{"id": "2", "get": {"pipeline": "some-pipeline-1"}},
								{"id": "3", "task": {"pipeline": "some-pipeline-2"}},
								{
									"id": "4",
									"on_success": {
										"step": {
											"id": "5", "put": {"pipeline": "some-pipeline-1"}
										},
										"on_success": {
											"id": "6", "dependent_get": {"pipeline": "some-pipeline-2"}
										}
									}
								}
							]
						}
					}`,
				)
				fakeTeamDB.GetPipelineByNameStub = func(pipelineName string) (db.SavedPipeline, error) {
					switch pipelineName {
					case "some-pipeline-1":
						return db.SavedPipeline{ID: 1}, nil
					case "some-pipeline-2":
						return db.SavedPipeline{ID: 2}, nil
					default:
						errMessage := fmt.Sprintf("unknown pipeline name `%s`", pipelineName)
						Fail(errMessage)
						return db.SavedPipeline{}, errors.New(errMessage)
					}
				}
			})

			It("sets pipeline ID for each plan", func() {
				foundBuild, err := execEngine.LookupBuild(logger, dbBuild)
				Expect(err).NotTo(HaveOccurred())
				type metadata struct {
					Plan atc.Plan
				}
				var foundMetadata metadata
				err = json.Unmarshal([]byte(foundBuild.Metadata()), &foundMetadata)
				Expect(err).NotTo(HaveOccurred())
				Expect(foundMetadata.Plan).To(Equal(atc.Plan{
					ID: "1",
					Do: &atc.DoPlan{
						{
							ID:  "2",
							Get: &atc.GetPlan{PipelineID: 1},
						},
						{
							ID:   "3",
							Task: &atc.TaskPlan{PipelineID: 2},
						},
						{
							ID: "4",
							OnSuccess: &atc.OnSuccessPlan{
								Step: atc.Plan{
									ID: "5",
									Put: &atc.PutPlan{
										PipelineID: 1,
									},
								},
								Next: atc.Plan{
									ID: "6",
									DependentGet: &atc.DependentGetPlan{
										PipelineID: 2,
									},
								},
							},
						},
					},
				}))
			})

			Context("when pipeline can not be found", func() {
				var disaster error
				BeforeEach(func() {
					dbBuild.EngineMetadataReturns(`{
						"Plan": {
							"id": "1",
							"task": {"pipeline": "unknown-pipeline"}
						}
					}`,
					)
					disaster = errors.New("oh dear")
					fakeTeamDB.GetPipelineByNameReturns(db.SavedPipeline{}, disaster)
				})

				It("returns an error", func() {
					foundBuild, err := execEngine.LookupBuild(logger, dbBuild)
					Expect(err).To(Equal(disaster))
					Expect(foundBuild).To(BeNil())
				})
			})

			Context("when build plan has pipeline name and pipeline ID", func() {
				BeforeEach(func() {
					dbBuild.EngineMetadataReturns(`{
						"Plan": {
							"id": "1",
							"task": {"pipeline": "some-pipeline","pipeline_id": 42}
						}
					}`,
					)
				})

				It("returns an error", func() {
					foundBuild, err := execEngine.LookupBuild(logger, dbBuild)
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring(
						"build plan with ID 1 has both pipeline name (some-pipeline) and ID (42)",
					))
					Expect(foundBuild).To(BeNil())
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
