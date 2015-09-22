package engine_test

import (
	"os"
	"time"

	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/engine"
	"github.com/concourse/atc/engine/fakes"
	"github.com/concourse/atc/exec"
	"github.com/concourse/atc/worker"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pivotal-golang/lager/lagertest"

	execfakes "github.com/concourse/atc/exec/fakes"
)

var _ = Describe("Exec Engine with Timeout", func() {

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

	Context("running timeout steps", func() {
		var (
			taskStepFactory *execfakes.FakeStepFactory
			taskStep        *execfakes.FakeStep

			inputStepFactory *execfakes.FakeStepFactory
			inputStep        *execfakes.FakeStep

			outputStepFactory *execfakes.FakeStepFactory
			outputStep        *execfakes.FakeStep

			dependentGetStepFactory *execfakes.FakeStepFactory
			dependentGetStep        *execfakes.FakeStep

			fakeDelegate       *fakes.FakeBuildDelegate
			fakeInputDelegate  *execfakes.FakeGetDelegate
			fakeOutputDelegate *execfakes.FakePutDelegate
			timeout            string
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

			dependentGetStepFactory = new(execfakes.FakeStepFactory)
			dependentGetStep = new(execfakes.FakeStep)
			dependentGetStep.ResultStub = successResult(true)
			dependentGetStepFactory.UsingReturns(dependentGetStep)
			fakeFactory.DependentGetReturns(dependentGetStepFactory)

			fakeDelegate = new(fakes.FakeBuildDelegate)
			fakeDelegateFactory.DelegateReturns(fakeDelegate)

			fakeInputDelegate = new(execfakes.FakeGetDelegate)
			fakeDelegate.InputDelegateReturns(fakeInputDelegate)

			fakeOutputDelegate = new(execfakes.FakePutDelegate)
			fakeDelegate.OutputDelegateReturns(fakeOutputDelegate)

		})

		Context("put", func() {

			Context("constructing steps", func() {

				It("constructs the step correctly", func() {
					plan := atc.Plan{
						OnSuccess: &atc.OnSuccessPlan{
							Step: atc.Plan{
								Location: &atc.Location{
									ParentID:      0,
									ID:            1,
									ParallelGroup: 0,
								},
								Put: &atc.PutPlan{
									Type:     "git",
									Name:     "some-put",
									Resource: "some-resource",
									Pipeline: "some-pipeline",
									Source: atc.Source{
										"uri": "git://some-resource",
									},
								},
							},
							Next: atc.Plan{
								Location: &atc.Location{
									ParentID:      1,
									ID:            2,
									ParallelGroup: 0,
								},
								DependentGet: &atc.DependentGetPlan{
									Type:     "git",
									Name:     "some-put",
									Resource: "some-resource",
									Pipeline: "some-pipeline",
									Source: atc.Source{
										"uri": "git://some-resource",
									},
								},
							},
						},
					}
					build, err := execEngine.CreateBuild(buildModel, plan)
					Ω(err).ShouldNot(HaveOccurred())
					build.Resume(logger)
					Ω(fakeFactory.PutCallCount()).Should(Equal(1))
					metadata, workerID, delegate, resourceConfig, _, _ := fakeFactory.PutArgsForCall(0)
					Ω(metadata).Should(Equal(expectedMetadata))
					Ω(workerID).Should(Equal(worker.Identifier{
						BuildID:      84,
						Type:         worker.ContainerTypePut,
						Name:         "some-put",
						StepLocation: 1,
					}))

					Ω(resourceConfig).Should(Equal(atc.ResourceConfig{
						Name: "some-resource",
						Type: "git",
						Source: atc.Source{
							"uri": "git://some-resource",
						},
					}))

					Ω(delegate).Should(Equal(fakeOutputDelegate))
					_, _, location := fakeDelegate.OutputDelegateArgsForCall(0)
					Ω(location).ShouldNot(BeNil())
				})

				Context("when the step times out", func() {
					BeforeEach(func() {
						outputStep.RunStub = func(signals <-chan os.Signal, ready chan<- struct{}) error {
							close(ready)

							time.Sleep(2 * time.Second)

							return nil
						}

						dependentGetStep.RunStub = func(signals <-chan os.Signal, ready chan<- struct{}) error {
							close(ready)

							time.Sleep(2 * time.Second)

							return nil
						}
					})
					It("does not run the next step", func() {
						plan := atc.Plan{
							OnSuccess: &atc.OnSuccessPlan{
								Step: atc.Plan{
									Timeout: &atc.TimeoutPlan{
										Duration: "3s",
										Step: atc.Plan{
											OnSuccess: &atc.OnSuccessPlan{
												Step: atc.Plan{
													Put: &atc.PutPlan{
														Name: "some-put",
													},
												},
												Next: atc.Plan{
													DependentGet: &atc.DependentGetPlan{
														Name: "some-dependent-get",
													},
												},
											},
										},
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
						Ω(build).Should(Equal(build))

						Ω(outputStep.RunCallCount()).Should(Equal(1))
						Ω(outputStep.ReleaseCallCount()).Should((BeNumerically(">", 0)))

						Ω(dependentGetStep.RunCallCount()).Should(Equal(1))
						Ω(dependentGetStep.ReleaseCallCount()).Should((BeNumerically(">", 0)))

						Ω(taskStep.RunCallCount()).Should(Equal(0))

						Ω(fakeDelegate.FinishCallCount()).Should(Equal(1))

						_, err, succeeded, aborted := fakeDelegate.FinishArgsForCall(0)
						Ω(err).ShouldNot(BeNil())
						Ω(err.Error()).Should(ContainSubstring(exec.ErrStepTimedOut.Error()))
						Ω(succeeded).Should(Equal(exec.Success(false)))
						Ω(aborted).Should(BeFalse())
					})
				})
			})

		})

		Context("get", func() {
			Context("constructing steps", func() {

				BeforeEach(func() {
					plan := atc.Plan{
						Timeout: &atc.TimeoutPlan{
							Duration: timeout,
							Step: atc.Plan{
								Location: &atc.Location{},
								Get: &atc.GetPlan{
									Name: "some-input",
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
					metadata, sourceName, workerID, delegate, _, _, _, _ := fakeFactory.GetArgsForCall(0)
					Ω(metadata).Should(Equal(expectedMetadata))
					Ω(sourceName).Should(Equal(exec.SourceName("some-input")))
					Ω(workerID).Should(Equal(worker.Identifier{
						BuildID: 84,
						Type:    worker.ContainerTypeGet,
						Name:    "some-input",
					}))

					Ω(delegate).Should(Equal(fakeInputDelegate))
					_, _, location := fakeDelegate.InputDelegateArgsForCall(0)
					Ω(location).ShouldNot(BeNil())
				})
			})

			Context("when the step times out", func() {
				BeforeEach(func() {
					inputStep.RunStub = func(signals <-chan os.Signal, ready chan<- struct{}) error {
						close(ready)

						time.Sleep(4 * time.Second)

						return nil
					}
				})

				It("does not run the next step", func() {
					plan := atc.Plan{
						OnSuccess: &atc.OnSuccessPlan{
							Step: atc.Plan{
								Timeout: &atc.TimeoutPlan{
									Duration: "2s",
									Step: atc.Plan{
										Location: &atc.Location{},
										Get: &atc.GetPlan{
											Name: "some-input",
										},
									},
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

					build, err := execEngine.CreateBuild(buildModel, plan)

					Ω(err).ShouldNot(HaveOccurred())

					build.Resume(logger)

					Ω(inputStep.RunCallCount()).Should(Equal(1))
					Ω(inputStep.ReleaseCallCount()).Should((BeNumerically(">", 0)))

					Ω(taskStep.RunCallCount()).Should(Equal(0))

					Ω(fakeDelegate.FinishCallCount()).Should(Equal(1))

					_, err, succeeded, aborted := fakeDelegate.FinishArgsForCall(0)
					Ω(err.Error()).Should(ContainSubstring(exec.ErrStepTimedOut.Error()))
					Ω(succeeded).Should(Equal(exec.Success(false)))
					Ω(aborted).Should(BeFalse())
				})
			})
		})
	})
})
