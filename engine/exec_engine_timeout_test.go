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
					Expect(err).NotTo(HaveOccurred())
					build.Resume(logger)
					Expect(fakeFactory.PutCallCount()).To(Equal(1))
					logger, metadata, workerID, delegate, resourceConfig, _, _ := fakeFactory.PutArgsForCall(0)
					Expect(logger).NotTo(BeNil())
					Expect(metadata).To(Equal(expectedMetadata))
					Expect(workerID).To(Equal(worker.Identifier{
						BuildID:      84,
						Type:         db.ContainerTypePut,
						Name:         "some-put",
						PipelineName: "some-pipeline",
						StepLocation: 1,
					}))

					Expect(resourceConfig).To(Equal(atc.ResourceConfig{
						Name: "some-resource",
						Type: "git",
						Source: atc.Source{
							"uri": "git://some-resource",
						},
					}))

					Expect(delegate).To(Equal(fakeOutputDelegate))
					_, _, location := fakeDelegate.OutputDelegateArgsForCall(0)
					Expect(location).NotTo(BeNil())
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
						Expect(err).NotTo(HaveOccurred())
						build.Resume(logger)
						Expect(build).To(Equal(build))

						Expect(outputStep.RunCallCount()).To(Equal(1))
						Expect(outputStep.ReleaseCallCount()).To((BeNumerically(">", 0)))

						Expect(dependentGetStep.RunCallCount()).To(Equal(1))
						Expect(dependentGetStep.ReleaseCallCount()).To((BeNumerically(">", 0)))

						Expect(taskStep.RunCallCount()).To(Equal(0))

						Expect(fakeDelegate.FinishCallCount()).To(Equal(1))

						_, err, succeeded, aborted := fakeDelegate.FinishArgsForCall(0)
						Expect(err).NotTo(BeNil())
						Expect(err.Error()).To(ContainSubstring(exec.ErrStepTimedOut.Error()))
						Expect(succeeded).To(Equal(exec.Success(false)))
						Expect(aborted).To(BeFalse())
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

					Expect(err).NotTo(HaveOccurred())

					build.Resume(logger)

					Expect(inputStep.RunCallCount()).To(Equal(1))
					Expect(inputStep.ReleaseCallCount()).To((BeNumerically(">", 0)))

					Expect(taskStep.RunCallCount()).To(Equal(0))

					Expect(fakeDelegate.FinishCallCount()).To(Equal(1))

					_, err, succeeded, aborted := fakeDelegate.FinishArgsForCall(0)
					Expect(err.Error()).To(ContainSubstring(exec.ErrStepTimedOut.Error()))
					Expect(succeeded).To(Equal(exec.Success(false)))
					Expect(aborted).To(BeFalse())
				})
			})
		})
	})
})
