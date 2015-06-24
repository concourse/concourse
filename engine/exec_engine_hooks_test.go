package engine_test

import (
	"os"

	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/engine"
	"github.com/concourse/atc/engine/fakes"
	execfakes "github.com/concourse/atc/exec/fakes"
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

	Context("when we have a plan with success hooks", func() {
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
