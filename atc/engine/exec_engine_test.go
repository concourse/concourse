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

var _ = Describe("ExecEngine", func() {
	var (
		fakeFactory         *execfakes.FakeFactory
		fakeDelegateFactory *enginefakes.FakeBuildDelegateFactory
		logger              *lagertest.TestLogger

		execEngine engine.Engine

		expectedTeamID     = 1111
		expectedPipelineID = 2222
		expectedJobID      = 3333
		expectedBuildID    = 4444
	)

	BeforeEach(func() {
		fakeFactory = new(execfakes.FakeFactory)
		fakeDelegateFactory = new(enginefakes.FakeBuildDelegateFactory)
		logger = lagertest.NewTestLogger("test")

		execEngine = engine.NewExecEngine(
			fakeFactory,
			fakeDelegateFactory,
			"http://example.com",
		)
	})

	Describe("Resume", func() {
		var (
			fakeDelegate *enginefakes.FakeBuildDelegate

			dbBuild          *dbfakes.FakeBuild
			expectedMetadata engine.StepMetadata

			outputPlan atc.Plan

			build engine.Build

			inputStep  *execfakes.FakeStep
			taskStep   *execfakes.FakeStep
			outputStep *execfakes.FakeStep

			planFactory atc.PlanFactory
		)

		BeforeEach(func() {
			planFactory = atc.NewPlanFactory(123)

			dbBuild = new(dbfakes.FakeBuild)
			dbBuild.IDReturns(expectedBuildID)
			dbBuild.NameReturns("42")
			dbBuild.JobNameReturns("some-job")
			dbBuild.JobIDReturns(expectedJobID)
			dbBuild.PipelineNameReturns("some-pipeline")
			dbBuild.PipelineIDReturns(expectedPipelineID)
			dbBuild.TeamNameReturns("some-team")
			dbBuild.TeamIDReturns(expectedTeamID)

			expectedMetadata = engine.StepMetadata{
				BuildID:      expectedBuildID,
				BuildName:    "42",
				JobName:      "some-job",
				PipelineName: "some-pipeline",
				TeamName:     "some-team",
				ExternalURL:  "http://example.com",
			}

			fakeDelegate = new(enginefakes.FakeBuildDelegate)
			fakeDelegateFactory.DelegateReturns(fakeDelegate)

			inputStep = new(execfakes.FakeStep)
			inputStep.SucceededReturns(true)
			fakeFactory.GetReturns(inputStep)

			taskStep = new(execfakes.FakeStep)
			taskStep.SucceededReturns(true)
			fakeFactory.TaskReturns(taskStep)

			outputStep = new(execfakes.FakeStep)
			outputStep.SucceededReturns(true)
			fakeFactory.PutReturns(outputStep)
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
				})

				otherPutPlan = planFactory.NewPlan(atc.PutPlan{
					Name:     "some-put-2",
					Resource: "some-output-resource-2",
					Type:     "put",
					Source:   atc.Source{"some": "source-2"},
					Params:   atc.Params{"some": "params-2"},
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
					build, err = execEngine.CreateBuild(logger, dbBuild, outputPlan)
					Expect(err).NotTo(HaveOccurred())

					build.Resume(logger)
					Expect(fakeFactory.PutCallCount()).To(Equal(2))

					logger, plan, build, stepMetadata, containerMetadata, _ := fakeFactory.PutArgsForCall(0)
					Expect(logger).NotTo(BeNil())
					Expect(build).To(Equal(dbBuild))
					Expect(plan).To(Equal(putPlan))
					Expect(stepMetadata).To(Equal(expectedMetadata))
					Expect(containerMetadata).To(Equal(db.ContainerMetadata{
						Type:         db.ContainerTypePut,
						StepName:     "some-put",
						PipelineID:   expectedPipelineID,
						PipelineName: "some-pipeline",
						JobID:        expectedJobID,
						JobName:      "some-job",
						BuildID:      expectedBuildID,
						BuildName:    "42",
					}))

					logger, plan, build, stepMetadata, containerMetadata, _ = fakeFactory.PutArgsForCall(1)
					Expect(logger).NotTo(BeNil())
					Expect(build).To(Equal(dbBuild))
					Expect(plan).To(Equal(otherPutPlan))
					Expect(containerMetadata).To(Equal(db.ContainerMetadata{
						Type:         db.ContainerTypePut,
						StepName:     "some-put-2",
						PipelineID:   expectedPipelineID,
						PipelineName: "some-pipeline",
						JobID:        expectedJobID,
						JobName:      "some-job",
						BuildID:      expectedBuildID,
						BuildName:    "42",
					}))
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
				})

				taskPlan = planFactory.NewPlan(atc.TaskPlan{
					Name:       "some-task",
					Privileged: false,
					Tags:       atc.Tags{"some", "task", "tags"},
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
				logger, plan, build, stepMetadata, containerMetadata, _ := fakeFactory.GetArgsForCall(0)
				Expect(logger).NotTo(BeNil())
				Expect(build).To(Equal(dbBuild))
				expectedPlan := getPlan
				expectedPlan.Attempts = []int{1}
				Expect(plan).To(Equal(expectedPlan))
				Expect(stepMetadata).To(Equal(expectedMetadata))
				Expect(containerMetadata).To(Equal(db.ContainerMetadata{
					Type:         db.ContainerTypeGet,
					StepName:     "some-get",
					PipelineID:   expectedPipelineID,
					PipelineName: "some-pipeline",
					JobID:        expectedJobID,
					JobName:      "some-job",
					BuildID:      expectedBuildID,
					BuildName:    "42",
					Attempt:      "1",
				}))
			})

			It("constructs the second get correctly", func() {
				logger, plan, build, stepMetadata, containerMetadata, _ := fakeFactory.GetArgsForCall(1)
				Expect(logger).NotTo(BeNil())
				Expect(build).To(Equal(dbBuild))
				expectedPlan := getPlan
				expectedPlan.Attempts = []int{3}
				Expect(plan).To(Equal(expectedPlan))
				Expect(stepMetadata).To(Equal(expectedMetadata))
				Expect(containerMetadata).To(Equal(db.ContainerMetadata{
					Type:         db.ContainerTypeGet,
					StepName:     "some-get",
					PipelineID:   expectedPipelineID,
					PipelineName: "some-pipeline",
					JobID:        expectedJobID,
					JobName:      "some-job",
					BuildID:      expectedBuildID,
					BuildName:    "42",
					Attempt:      "3",
				}))
			})

			It("constructs nested retries correctly", func() {
				Expect(*retryPlanTwo.Retry).To(HaveLen(2))
			})

			It("constructs nested steps correctly", func() {
				logger, plan, build, containerMetadata, _ := fakeFactory.TaskArgsForCall(0)
				Expect(logger).NotTo(BeNil())
				Expect(build).To(Equal(dbBuild))
				expectedPlan := taskPlan
				expectedPlan.Attempts = []int{2, 1}
				Expect(plan).To(Equal(expectedPlan))
				Expect(containerMetadata).To(Equal(db.ContainerMetadata{
					Type:         db.ContainerTypeTask,
					StepName:     "some-task",
					PipelineID:   expectedPipelineID,
					PipelineName: "some-pipeline",
					JobID:        expectedJobID,
					JobName:      "some-job",
					BuildID:      expectedBuildID,
					BuildName:    "42",
					Attempt:      "2.1",
				}))

				logger, plan, build, containerMetadata, _ = fakeFactory.TaskArgsForCall(1)
				Expect(logger).NotTo(BeNil())
				Expect(build).To(Equal(dbBuild))
				expectedPlan = taskPlan
				expectedPlan.Attempts = []int{2, 2}
				Expect(plan).To(Equal(expectedPlan))
				Expect(containerMetadata).To(Equal(db.ContainerMetadata{
					Type:         db.ContainerTypeTask,
					StepName:     "some-task",
					PipelineID:   expectedPipelineID,
					PipelineName: "some-pipeline",
					JobID:        expectedJobID,
					JobName:      "some-job",
					BuildID:      expectedBuildID,
					BuildName:    "42",
					Attempt:      "2.2",
				}))
			})
		})

		Context("with a plan where conditional steps are inside retries", func() {
			var (
				retryPlan     atc.Plan
				onAbortPlan   atc.Plan
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
					ConfigPath: "some-config-path",
				})

				onAbortPlan = planFactory.NewPlan(atc.OnAbortPlan{
					Step: leafPlan,
					Next: leafPlan,
				})

				onSuccessPlan = planFactory.NewPlan(atc.OnSuccessPlan{
					Step: onAbortPlan,
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
				Expect(fakeFactory.TaskCallCount()).To(Equal(5))
			})

			It("constructs nested steps correctly", func() {
				_, _, _, containerMetadata, _ := fakeFactory.TaskArgsForCall(0)
				Expect(containerMetadata.Attempt).To(Equal("1"))
				_, _, _, containerMetadata, _ = fakeFactory.TaskArgsForCall(1)
				Expect(containerMetadata.Attempt).To(Equal("1"))
				_, _, _, containerMetadata, _ = fakeFactory.TaskArgsForCall(2)
				Expect(containerMetadata.Attempt).To(Equal("1"))
				_, _, _, containerMetadata, _ = fakeFactory.TaskArgsForCall(3)
				Expect(containerMetadata.Attempt).To(Equal("1"))
				_, _, _, containerMetadata, _ = fakeFactory.TaskArgsForCall(4)
				Expect(containerMetadata.Attempt).To(Equal("1"))
			})
		})

		Context("with a basic plan", func() {
			var expectedPlan atc.Plan

			Context("that contains inputs", func() {
				BeforeEach(func() {
					expectedPlan = planFactory.NewPlan(atc.GetPlan{
						Name:     "some-input",
						Resource: "some-input-resource",
						Type:     "get",
						Tags:     []string{"some", "get", "tags"},
						Version:  &atc.Version{"some": "version"},
						Source:   atc.Source{"some": "source"},
						Params:   atc.Params{"some": "params"},
					})
				})

				It("constructs inputs correctly", func() {
					var err error
					build, err := execEngine.CreateBuild(logger, dbBuild, expectedPlan)
					Expect(err).NotTo(HaveOccurred())

					build.Resume(logger)
					Expect(fakeFactory.GetCallCount()).To(Equal(1))

					logger, plan, dBuild, stepMetadata, containerMetadata, _ := fakeFactory.GetArgsForCall(0)
					Expect(logger).NotTo(BeNil())
					Expect(dBuild).To(Equal(dbBuild))
					Expect(plan).To(Equal(expectedPlan))
					Expect(stepMetadata).To(Equal(expectedMetadata))
					Expect(containerMetadata).To(Equal(db.ContainerMetadata{
						Type:         db.ContainerTypeGet,
						StepName:     "some-input",
						PipelineID:   expectedPipelineID,
						PipelineName: "some-pipeline",
						JobID:        expectedJobID,
						JobName:      "some-job",
						BuildID:      expectedBuildID,
						BuildName:    "42",
					}))
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
						InputMapping:  inputMapping,
						OutputMapping: outputMapping,
					}
				})

				JustBeforeEach(func() {
					expectedPlan = planFactory.NewPlan(taskPlan)
				})

				It("constructs tasks correctly", func() {
					var err error
					build, err = execEngine.CreateBuild(logger, dbBuild, expectedPlan)
					Expect(err).NotTo(HaveOccurred())

					build.Resume(logger)
					Expect(fakeFactory.TaskCallCount()).To(Equal(1))

					logger, plan, build, containerMetadata, _ := fakeFactory.TaskArgsForCall(0)
					Expect(logger).NotTo(BeNil())
					Expect(build).To(Equal(dbBuild))
					Expect(plan).To(Equal(expectedPlan))
					Expect(containerMetadata).To(Equal(db.ContainerMetadata{
						Type:         db.ContainerTypeTask,
						StepName:     "some-task",
						PipelineID:   expectedPipelineID,
						PipelineName: "some-pipeline",
						JobID:        expectedJobID,
						JobName:      "some-job",
						BuildID:      expectedBuildID,
						BuildName:    "42",
					}))
				})
			})

			Context("that contains outputs", func() {
				var (
					expectedPlan     atc.Plan
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
					})
					dependentGetPlan = planFactory.NewPlan(atc.GetPlan{
						Name:        "some-get",
						Resource:    "some-input-resource",
						Tags:        []string{"some", "putget", "tags"},
						Type:        "get",
						VersionFrom: &putPlan.ID,
						Source:      atc.Source{"some": "source"},
						Params:      atc.Params{"another": "params"},
					})

					expectedPlan = planFactory.NewPlan(atc.OnSuccessPlan{
						Step: putPlan,
						Next: dependentGetPlan,
					})
				})

				It("constructs the put correctly", func() {
					var err error
					build, err = execEngine.CreateBuild(logger, dbBuild, expectedPlan)
					Expect(err).NotTo(HaveOccurred())

					build.Resume(logger)
					Expect(fakeFactory.PutCallCount()).To(Equal(1))

					logger, plan, build, stepMetadata, containerMetadata, _ := fakeFactory.PutArgsForCall(0)
					Expect(logger).NotTo(BeNil())
					Expect(build).To(Equal(dbBuild))
					Expect(plan).To(Equal(putPlan))
					Expect(stepMetadata).To(Equal(expectedMetadata))
					Expect(containerMetadata).To(Equal(db.ContainerMetadata{
						Type:         db.ContainerTypePut,
						StepName:     "some-put",
						PipelineID:   expectedPipelineID,
						PipelineName: "some-pipeline",
						JobID:        expectedJobID,
						JobName:      "some-job",
						BuildID:      expectedBuildID,
						BuildName:    "42",
					}))
				})

				It("constructs the dependent get correctly", func() {
					var err error
					build, err = execEngine.CreateBuild(logger, dbBuild, expectedPlan)
					Expect(err).NotTo(HaveOccurred())

					build.Resume(logger)
					Expect(fakeFactory.GetCallCount()).To(Equal(1))

					logger, plan, build, stepMetadata, containerMetadata, _ := fakeFactory.GetArgsForCall(0)
					Expect(logger).NotTo(BeNil())
					Expect(build).To(Equal(dbBuild))
					Expect(plan).To(Equal(dependentGetPlan))
					Expect(stepMetadata).To(Equal(expectedMetadata))
					Expect(containerMetadata).To(Equal(db.ContainerMetadata{
						Type:         db.ContainerTypeGet,
						StepName:     "some-get",
						PipelineID:   expectedPipelineID,
						PipelineName: "some-pipeline",
						JobID:        expectedJobID,
						JobName:      "some-job",
						BuildID:      expectedBuildID,
						BuildName:    "42",
					}))
				})
			})
		})
	})

	Describe("LookupBuild", func() {
		var dbBuild *dbfakes.FakeBuild

		BeforeEach(func() {
			dbBuild = new(dbfakes.FakeBuild)
			dbBuild.IDReturns(expectedBuildID)
			dbBuild.NameReturns("42")
			dbBuild.JobNameReturns("some-job")
			dbBuild.JobIDReturns(expectedJobID)
			dbBuild.PipelineNameReturns("some-pipeline")
			dbBuild.PipelineIDReturns(expectedPipelineID)
			dbBuild.TeamNameReturns("some-team")
			dbBuild.TeamIDReturns(expectedTeamID)
		})

		Context("when the build has a get step", func() {
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
									"pipeline_id": 2222
								}
							}
						}`,
				)

				fakeDelegate := new(enginefakes.FakeBuildDelegate)
				fakeDelegateFactory.DelegateReturns(fakeDelegate)

				inputStep := new(execfakes.FakeStep)
				inputStep.SucceededReturns(true)
				fakeFactory.GetReturns(inputStep)
			})

			It("constructs the get correctly", func() {
				foundBuild, err := execEngine.LookupBuild(logger, dbBuild)
				Expect(err).NotTo(HaveOccurred())

				foundBuild.Resume(logger)
				Expect(fakeFactory.GetCallCount()).To(Equal(1))
				logger, plan, build, stepMetadata, containerMetadata, _ := fakeFactory.GetArgsForCall(0)
				Expect(logger).NotTo(BeNil())
				Expect(build).To(Equal(dbBuild))
				Expect(plan.ID).To(Equal(atc.PlanID("47")))
				Expect(stepMetadata).To(Equal(engine.StepMetadata{
					BuildID:      expectedBuildID,
					BuildName:    "42",
					JobName:      "some-job",
					PipelineName: "some-pipeline",
					TeamName:     "some-team",
					ExternalURL:  "http://example.com",
				}))
				Expect(containerMetadata).To(Equal(db.ContainerMetadata{
					Type:         db.ContainerTypeGet,
					StepName:     "some-get",
					PipelineID:   expectedPipelineID,
					PipelineName: "some-pipeline",
					JobID:        expectedJobID,
					JobName:      "some-job",
					BuildID:      expectedBuildID,
					BuildName:    "42",
					Attempt:      "1",
				}))
			})
		})

		Context("when engine metadata is empty", func() {
			BeforeEach(func() {
				dbBuild.EngineMetadataReturns("{}")

				fakeDelegate := new(enginefakes.FakeBuildDelegate)
				fakeDelegateFactory.DelegateReturns(fakeDelegate)

				inputStep := new(execfakes.FakeStep)
				inputStep.SucceededReturns(true)
				fakeFactory.GetReturns(inputStep)
			})

			It("does not error", func() {
				_, err := execEngine.LookupBuild(logger, dbBuild)
				Expect(err).NotTo(HaveOccurred())
			})
		})
	})
})
