package engine_test

import (
	"errors"
	"os"

	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/engine"
	"github.com/concourse/atc/engine/fakes"
	"github.com/concourse/atc/exec"
	execfakes "github.com/concourse/atc/exec/fakes"
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
			fakeExecutionDelegate *execfakes.FakeExecuteDelegate
			fakeOutputDelegate    *execfakes.FakePutDelegate

			buildModel db.Build

			inputPlan   *atc.GetPlan
			outputPlan  *atc.ConditionalPlan
			privileged  bool
			buildConfig *atc.BuildConfig

			buildConfigPath string

			build engine.Build

			logger *lagertest.TestLogger

			inputStep   *execfakes.FakeStep
			inputSource *execfakes.FakeArtifactSource

			executeStep   *execfakes.FakeStep
			executeSource *execfakes.FakeArtifactSource

			outputStep   *execfakes.FakeStep
			outputSource *execfakes.FakeArtifactSource
		)

		BeforeEach(func() {
			logger = lagertest.NewTestLogger("test")

			buildModel = db.Build{ID: 42}

			buildConfig = &atc.BuildConfig{
				Image:  "some-image",
				Params: map[string]string{"PARAM": "value"},
				Run: atc.BuildRunConfig{
					Path: "some-path",
					Args: []string{"some", "args"},
				},
				Inputs: []atc.BuildInputConfig{
					{Name: "some-input"},
				},
			}

			buildConfigPath = "some-input/build.yml"

			inputPlan = &atc.GetPlan{
				Name:     "some-input",
				Resource: "some-input-resource",
				Type:     "some-type",
				Version:  atc.Version{"some": "version"},
				Source:   atc.Source{"some": "source"},
				Params:   atc.Params{"some": "params"},
			}

			outputPlan = &atc.ConditionalPlan{
				Conditions: atc.Conditions{atc.ConditionSuccess},
				Plan: atc.Plan{
					Put: &atc.PutPlan{
						Resource: "some-output-resource",
						Type:     "some-type",
						Source:   atc.Source{"some": "source"},
						Params:   atc.Params{"some": "params"},
					},
				},
			}

			privileged = false

			fakeDelegate = new(fakes.FakeBuildDelegate)
			fakeDelegateFactory.DelegateReturns(fakeDelegate)

			fakeInputDelegate = new(execfakes.FakeGetDelegate)
			fakeDelegate.InputDelegateReturns(fakeInputDelegate)

			fakeExecutionDelegate = new(execfakes.FakeExecuteDelegate)
			fakeDelegate.ExecutionDelegateReturns(fakeExecutionDelegate)

			fakeOutputDelegate = new(execfakes.FakePutDelegate)
			fakeDelegate.OutputDelegateReturns(fakeOutputDelegate)

			inputStep = new(execfakes.FakeStep)
			inputSource = new(execfakes.FakeArtifactSource)
			inputStep.UsingReturns(inputSource)
			fakeFactory.GetReturns(inputStep)

			executeStep = new(execfakes.FakeStep)
			executeSource = new(execfakes.FakeArtifactSource)
			executeSource.ResultStub = successResult(true)
			executeStep.UsingReturns(executeSource)
			fakeFactory.ExecuteReturns(executeStep)

			outputStep = new(execfakes.FakeStep)
			outputSource = new(execfakes.FakeArtifactSource)
			outputStep.UsingReturns(outputSource)
			fakeFactory.PutReturns(outputStep)
		})

		JustBeforeEach(func() {
			var err error
			build, err = execEngine.CreateBuild(buildModel, atc.Plan{
				Compose: &atc.ComposePlan{
					A: atc.Plan{
						Aggregate: &atc.AggregatePlan{
							"some-input": atc.Plan{
								Get: inputPlan,
							},
						},
					},
					B: atc.Plan{
						Compose: &atc.ComposePlan{
							A: atc.Plan{
								Execute: &atc.ExecutePlan{
									Privileged: privileged,

									Config:     buildConfig,
									ConfigPath: buildConfigPath,
								},
							},
							B: atc.Plan{
								Aggregate: &atc.AggregatePlan{
									"some-output-resource": atc.Plan{
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

		It("constructs inputs correctly", func() {
			Ω(fakeFactory.GetCallCount()).Should(Equal(1))

			sessionID, delegate, resourceConfig, params, version := fakeFactory.GetArgsForCall(0)
			Ω(sessionID).Should(Equal(exec.SessionID("build-42-input-some-input")))
			Ω(delegate).Should(Equal(fakeInputDelegate))
			Ω(resourceConfig.Name).Should(Equal("some-input-resource"))
			Ω(resourceConfig.Type).Should(Equal("some-type"))
			Ω(resourceConfig.Source).Should(Equal(atc.Source{"some": "source"}))
			Ω(params).Should(Equal(atc.Params{"some": "params"}))
			Ω(version).Should(Equal(atc.Version{"some": "version"}))
		})

		It("constructs outputs correctly", func() {
			Ω(fakeFactory.PutCallCount()).Should(Equal(1))

			sessionID, delegate, resourceConfig, params := fakeFactory.PutArgsForCall(0)
			Ω(sessionID).Should(Equal(exec.SessionID("build-42-output-some-output-resource")))
			Ω(delegate).Should(Equal(fakeOutputDelegate))
			Ω(resourceConfig.Name).Should(Equal("some-output-resource"))
			Ω(resourceConfig.Type).Should(Equal("some-type"))
			Ω(resourceConfig.Source).Should(Equal(atc.Source{"some": "source"}))
			Ω(params).Should(Equal(atc.Params{"some": "params"}))
		})

		It("constructs executions correctly", func() {
			Ω(fakeFactory.ExecuteCallCount()).Should(Equal(1))

			sessionID, delegate, privileged, configSource := fakeFactory.ExecuteArgsForCall(0)
			Ω(sessionID).Should(Equal(exec.SessionID("build-42-execute")))
			Ω(delegate).Should(Equal(fakeExecutionDelegate))
			Ω(privileged).Should(Equal(exec.Privileged(false)))
			Ω(configSource).ShouldNot(BeNil())
		})

		Context("when the steps complete", func() {
			BeforeEach(func() {
				assertNotReleased := func(signals <-chan os.Signal, ready chan<- struct{}) error {
					defer GinkgoRecover()
					Consistently(inputSource.ReleaseCallCount).Should(BeZero())
					Consistently(executeSource.ReleaseCallCount).Should(BeZero())
					Consistently(outputSource.ReleaseCallCount).Should(BeZero())
					return nil
				}

				inputSource.RunStub = assertNotReleased
				executeSource.RunStub = assertNotReleased
				outputSource.RunStub = assertNotReleased
			})

			It("releases all sources", func() {
				Ω(inputSource.ReleaseCallCount()).Should(Equal(1))
				Ω(executeSource.ReleaseCallCount()).Should(Equal(1))
				Ω(outputSource.ReleaseCallCount()).Should(Equal(1))
			})
		})

		Context("when the build is privileged", func() {
			BeforeEach(func() {
				privileged = true
			})

			It("constructs the execute step privileged", func() {
				Ω(fakeFactory.ExecuteCallCount()).Should(Equal(1))

				_, _, privileged, _ := fakeFactory.ExecuteArgsForCall(0)
				Ω(privileged).Should(Equal(exec.Privileged(true)))
			})
		})

		Context("when the input succeeds", func() {
			BeforeEach(func() {
				inputSource.RunReturns(nil)
			})

			Context("when executing the build errors", func() {
				disaster := errors.New("oh no!")

				BeforeEach(func() {
					executeSource.RunReturns(disaster)
				})

				It("does not execute any outputs", func() {
					Ω(outputSource.RunCallCount()).Should(BeZero())
				})

				It("finishes with error", func() {
					Ω(fakeDelegate.FinishCallCount()).Should(Equal(1))
					_, cbErr := fakeDelegate.FinishArgsForCall(0)
					Ω(cbErr).Should(MatchError(ContainSubstring(disaster.Error())))
				})
			})

			Context("when executing the build succeeds", func() {
				BeforeEach(func() {
					executeSource.RunReturns(nil)
					executeSource.ResultStub = successResult(true)
				})

				Context("when the output should perform on success", func() {
					BeforeEach(func() {
						outputPlan.Conditions = atc.Conditions{atc.ConditionSuccess}
					})

					It("executes the output", func() {
						Ω(outputSource.RunCallCount()).Should(Equal(1))
					})

					Context("when the output succeeds", func() {
						BeforeEach(func() {
							outputSource.RunReturns(nil)
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
							outputSource.RunReturns(disaster)
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

					It("does not execute the output", func() {
						Ω(outputSource.RunCallCount()).Should(BeZero())
					})
				})

				Context("when the output should perform on success or failure", func() {
					BeforeEach(func() {
						outputPlan.Conditions = atc.Conditions{atc.ConditionSuccess, atc.ConditionFailure}
					})

					It("executes the output", func() {
						Ω(outputSource.RunCallCount()).Should(Equal(1))
					})

					Context("when the output succeeds", func() {
						BeforeEach(func() {
							outputSource.RunReturns(nil)
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
							outputSource.RunReturns(disaster)
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

					It("does not execute the output", func() {
						Ω(outputSource.RunCallCount()).Should(BeZero())
					})
				})
			})

			Context("when executing the build fails", func() {
				BeforeEach(func() {
					executeSource.RunReturns(nil)
					executeSource.ResultStub = successResult(false)
				})

				Context("when the output should perform on success", func() {
					BeforeEach(func() {
						outputPlan.Conditions = atc.Conditions{atc.ConditionSuccess}
					})

					It("does not execute the output", func() {
						Ω(outputSource.RunCallCount()).Should(BeZero())
					})
				})

				Context("when the output should perform on failure", func() {
					BeforeEach(func() {
						outputPlan.Conditions = atc.Conditions{atc.ConditionFailure}
					})

					It("executes the output", func() {
						Ω(outputSource.RunCallCount()).Should(Equal(1))
					})

					Context("when the output succeeds", func() {
						BeforeEach(func() {
							outputSource.RunReturns(nil)
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
							outputSource.RunReturns(disaster)
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

					It("executes the output", func() {
						Ω(outputSource.RunCallCount()).Should(Equal(1))
					})

					Context("when the output succeeds", func() {
						BeforeEach(func() {
							outputSource.RunReturns(nil)
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
							outputSource.RunReturns(disaster)
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

					It("does not execute the output", func() {
						Ω(outputSource.RunCallCount()).Should(BeZero())
					})
				})
			})
		})

		Context("when an input errors", func() {
			disaster := errors.New("oh no!")

			BeforeEach(func() {
				inputSource.RunReturns(disaster)
			})

			It("does not execute the build", func() {
				Ω(executeSource.RunCallCount()).Should(BeZero())
			})

			It("does not execute any outputs", func() {
				Ω(outputSource.RunCallCount()).Should(BeZero())
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
