package engine_test

import (
	"bytes"
	"errors"
	"io/ioutil"

	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/engine"
	"github.com/concourse/atc/engine/fakes"
	"github.com/concourse/atc/event"
	"github.com/concourse/atc/exec"
	execfakes "github.com/concourse/atc/exec/fakes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pivotal-golang/lager"
	"github.com/pivotal-golang/lager/lagertest"
)

var _ = Describe("GardenEngine", func() {
	var (
		fakeFactory         *execfakes.FakeFactory
		fakeDelegateFactory *fakes.FakeBuildDelegateFactory
		fakeDB              *fakes.FakeEngineDB

		gardenEngine engine.Engine
	)

	BeforeEach(func() {
		fakeFactory = new(execfakes.FakeFactory)
		fakeDelegateFactory = new(fakes.FakeBuildDelegateFactory)
		fakeDB = new(fakes.FakeEngineDB)

		gardenEngine = engine.NewGardenEngine(fakeFactory, fakeDelegateFactory, fakeDB)
	})

	Describe("Resume", func() {
		var (
			fakeDelegate                  *fakes.FakeBuildDelegate
			fakeInputCompleteCallback     *execfakes.FakeCompleteCallback
			fakeExecutionCompleteCallback *execfakes.FakeCompleteCallback
			fakeOutputCompleteCallback    *execfakes.FakeCompleteCallback
			fakeFinishCallback            *execfakes.FakeCompleteCallback

			buildModel db.Build

			inputPlan   *atc.InputPlan
			outputPlan  *atc.OutputPlan
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

			inputPlan = &atc.InputPlan{
				Name:     "some-input",
				Resource: "some-input-resource",
				Type:     "some-type",
				Version:  atc.Version{"some": "version"},
				Source:   atc.Source{"some": "source"},
				Params:   atc.Params{"some": "params"},
			}

			outputPlan = &atc.OutputPlan{
				Name:   "some-output-resource",
				Type:   "some-type",
				On:     atc.OutputConditions{atc.OutputConditionSuccess},
				Source: atc.Source{"some": "source"},
				Params: atc.Params{"some": "params"},
			}

			privileged = false

			fakeDelegate = new(fakes.FakeBuildDelegate)
			fakeDelegateFactory.DelegateReturns(fakeDelegate)

			fakeInputCompleteCallback = new(execfakes.FakeCompleteCallback)
			fakeDelegate.InputCompletedReturns(fakeInputCompleteCallback)

			fakeExecutionCompleteCallback = new(execfakes.FakeCompleteCallback)
			fakeDelegate.ExecutionCompletedReturns(fakeExecutionCompleteCallback)

			fakeOutputCompleteCallback = new(execfakes.FakeCompleteCallback)
			fakeDelegate.OutputCompletedReturns(fakeOutputCompleteCallback)

			fakeFinishCallback = new(execfakes.FakeCompleteCallback)
			fakeDelegate.FinishReturns(fakeFinishCallback)

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
			build, err = gardenEngine.CreateBuild(buildModel, atc.BuildPlan{
				Privileged: privileged,

				Config:     buildConfig,
				ConfigPath: buildConfigPath,

				Inputs: []atc.InputPlan{*inputPlan},

				Outputs: []atc.OutputPlan{*outputPlan},
			})
			Ω(err).ShouldNot(HaveOccurred())

			build.Resume(logger)
		})

		It("constructs inputs correcly", func() {
			Ω(fakeFactory.GetCallCount()).Should(Equal(1))

			sessionID, ioConfig, resourceConfig, params, version := fakeFactory.GetArgsForCall(0)
			Ω(sessionID).Should(Equal(exec.SessionID("build-42-input-some-input")))
			Ω(resourceConfig.Name).Should(Equal("some-input-resource"))
			Ω(resourceConfig.Type).Should(Equal("some-type"))
			Ω(resourceConfig.Source).Should(Equal(atc.Source{"some": "source"}))
			Ω(params).Should(Equal(atc.Params{"some": "params"}))
			Ω(version).Should(Equal(atc.Version{"some": "version"}))

			Ω(fakeDB.SaveBuildEventCallCount()).Should(BeZero())

			_, err := ioConfig.Stdout.Write([]byte("some stdout"))
			Ω(err).ShouldNot(HaveOccurred())

			_, err = ioConfig.Stderr.Write([]byte("some stderr"))
			Ω(err).ShouldNot(HaveOccurred())

			Ω(fakeDB.SaveBuildEventCallCount()).Should(Equal(2))

			buildID, savedEvent := fakeDB.SaveBuildEventArgsForCall(0)
			Ω(buildID).Should(Equal(buildModel.ID))
			Ω(savedEvent).Should(Equal(event.Log{
				Origin: event.Origin{
					Type: event.OriginTypeInput,
					Name: "some-input",
				},
				Payload: "some stdout",
			}))

			buildID, savedEvent = fakeDB.SaveBuildEventArgsForCall(1)
			Ω(buildID).Should(Equal(buildModel.ID))
			Ω(savedEvent).Should(Equal(event.Log{
				Origin: event.Origin{
					Type: event.OriginTypeInput,
					Name: "some-input",
				},
				Payload: "some stderr",
			}))
		})

		It("constructs outputs correcly", func() {
			Ω(fakeFactory.PutCallCount()).Should(Equal(1))

			sessionID, ioConfig, resourceConfig, params := fakeFactory.PutArgsForCall(0)
			Ω(sessionID).Should(Equal(exec.SessionID("build-42-output-some-output-resource")))
			Ω(resourceConfig.Name).Should(Equal("some-output-resource"))
			Ω(resourceConfig.Type).Should(Equal("some-type"))
			Ω(resourceConfig.Source).Should(Equal(atc.Source{"some": "source"}))
			Ω(params).Should(Equal(atc.Params{"some": "params"}))

			Ω(fakeDB.SaveBuildEventCallCount()).Should(BeZero())

			_, err := ioConfig.Stdout.Write([]byte("some stdout"))
			Ω(err).ShouldNot(HaveOccurred())

			_, err = ioConfig.Stderr.Write([]byte("some stderr"))
			Ω(err).ShouldNot(HaveOccurred())

			Ω(fakeDB.SaveBuildEventCallCount()).Should(Equal(2))

			buildID, savedEvent := fakeDB.SaveBuildEventArgsForCall(0)
			Ω(buildID).Should(Equal(buildModel.ID))
			Ω(savedEvent).Should(Equal(event.Log{
				Origin: event.Origin{
					Type: event.OriginTypeOutput,
					Name: "some-output-resource",
				},
				Payload: "some stdout",
			}))

			buildID, savedEvent = fakeDB.SaveBuildEventArgsForCall(1)
			Ω(buildID).Should(Equal(buildModel.ID))
			Ω(savedEvent).Should(Equal(event.Log{
				Origin: event.Origin{
					Type: event.OriginTypeOutput,
					Name: "some-output-resource",
				},
				Payload: "some stderr",
			}))
		})

		It("constructs executions correctly", func() {
			Ω(fakeFactory.ExecuteCallCount()).Should(Equal(1))

			sessionID, ioConfig, privileged, configSource := fakeFactory.ExecuteArgsForCall(0)
			Ω(sessionID).Should(Equal(exec.SessionID("build-42-execute")))
			Ω(privileged).Should(Equal(exec.Privileged(false)))
			Ω(configSource).ShouldNot(BeNil())

			Ω(fakeDB.SaveBuildEventCallCount()).Should(BeZero())

			_, err := ioConfig.Stdout.Write([]byte("some stdout"))
			Ω(err).ShouldNot(HaveOccurred())

			_, err = ioConfig.Stderr.Write([]byte("some stderr"))
			Ω(err).ShouldNot(HaveOccurred())

			Ω(fakeDB.SaveBuildEventCallCount()).Should(Equal(2))

			buildID, savedEvent := fakeDB.SaveBuildEventArgsForCall(0)
			Ω(buildID).Should(Equal(buildModel.ID))
			Ω(savedEvent).Should(Equal(event.Log{
				Origin: event.Origin{
					Type: event.OriginTypeRun,
					Name: "stdout",
				},
				Payload: "some stdout",
			}))

			buildID, savedEvent = fakeDB.SaveBuildEventArgsForCall(1)
			Ω(buildID).Should(Equal(buildModel.ID))
			Ω(savedEvent).Should(Equal(event.Log{
				Origin: event.Origin{
					Type: event.OriginTypeRun,
					Name: "stderr",
				},
				Payload: "some stderr",
			}))
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

		Describe("fetching the config", func() {
			It("emits an initialize event", func() {
				Ω(fakeFactory.ExecuteCallCount()).Should(Equal(1))

				_, _, _, configSource := fakeFactory.ExecuteArgsForCall(0)

				fakeSource := new(execfakes.FakeArtifactSource)

				configStream := ioutil.NopCloser(bytes.NewBufferString(`---
params:
  ANOTHER_PARAM: another-value
`))

				fakeSource.StreamFileReturns(configStream, nil)

				Ω(fakeDB.SaveBuildEventCallCount()).Should(BeZero())

				config, err := configSource.FetchConfig(fakeSource)
				Ω(err).ShouldNot(HaveOccurred())

				mergedConfig := *buildConfig
				mergedConfig.Params["ANOTHER_PARAM"] = "another-value"

				Ω(config).Should(Equal(mergedConfig))

				Ω(fakeDB.SaveBuildEventCallCount()).Should(Equal(1))

				buildID, savedEvent := fakeDB.SaveBuildEventArgsForCall(0)
				Ω(buildID).Should(Equal(buildModel.ID))
				Ω(savedEvent).Should(Equal(event.Initialize{config}))
			})
		})

		Context("before executing anything", func() {
			BeforeEach(func() {
				fakeDelegate.StartStub = func(lager.Logger) {
					Ω(inputSource.RunCallCount()).Should(BeZero())
					Ω(executeSource.RunCallCount()).Should(BeZero())
					Ω(outputSource.RunCallCount()).Should(BeZero())
				}
			})

			It("starts the build via the delegate", func() {
				Ω(fakeDelegate.StartCallCount()).Should(Equal(1))
				Ω(inputSource.RunCallCount()).Should(Equal(1))
				Ω(executeSource.RunCallCount()).Should(Equal(1))
				Ω(outputSource.RunCallCount()).Should(Equal(1))
			})
		})

		Context("when the input succeeds", func() {
			BeforeEach(func() {
				inputSource.RunReturns(nil)
			})

			It("invokes the delegate's input complete callback", func() {
				Ω(fakeInputCompleteCallback.CallCallCount()).Should(Equal(1))
				cbErr, cbSource := fakeInputCompleteCallback.CallArgsForCall(0)
				Ω(cbErr).ShouldNot(HaveOccurred())
				Ω(cbSource).Should(Equal(inputSource))
			})

			Context("when executing the build errors", func() {
				disaster := errors.New("oh no!")

				BeforeEach(func() {
					executeSource.RunReturns(disaster)
				})

				It("does not execute any outputs", func() {
					Ω(outputSource.RunCallCount()).Should(BeZero())
				})

				It("invokes the delegate's execution complete callback", func() {
					Ω(fakeExecutionCompleteCallback.CallCallCount()).Should(Equal(1))
					cbErr, cbSource := fakeExecutionCompleteCallback.CallArgsForCall(0)
					Ω(cbErr).Should(Equal(disaster))
					Ω(cbSource).Should(Equal(executeSource))
				})

				It("invokes the delegate's finish callback", func() {
					Ω(fakeFinishCallback.CallCallCount()).Should(Equal(1))
					cbErr, _ := fakeFinishCallback.CallArgsForCall(0)
					Ω(cbErr).Should(MatchError(ContainSubstring(disaster.Error())))
				})
			})

			Context("when executing the build succeeds", func() {
				BeforeEach(func() {
					executeSource.RunReturns(nil)
					executeSource.ResultStub = successResult(true)
				})

				It("invokes the delegate's execution complete callback", func() {
					Ω(fakeExecutionCompleteCallback.CallCallCount()).Should(Equal(1))
					cbErr, cbSource := fakeExecutionCompleteCallback.CallArgsForCall(0)
					Ω(cbErr).ShouldNot(HaveOccurred())
					Ω(cbSource).Should(Equal(executeSource))
				})

				Context("when the output should perform on success", func() {
					BeforeEach(func() {
						outputPlan.On = atc.OutputConditions{atc.OutputConditionSuccess}
					})

					It("executes the output", func() {
						Ω(outputSource.RunCallCount()).Should(Equal(1))
					})

					Context("when the output succeeds", func() {
						BeforeEach(func() {
							outputSource.RunReturns(nil)
						})

						It("invokes the delegate's output complete callback", func() {
							Ω(fakeOutputCompleteCallback.CallCallCount()).Should(Equal(1))
							cbErr, cbSource := fakeOutputCompleteCallback.CallArgsForCall(0)
							Ω(cbErr).ShouldNot(HaveOccurred())
							Ω(cbSource).Should(Equal(outputSource))
						})
					})

					Context("when the output fails", func() {
						disaster := errors.New("oh no!")

						BeforeEach(func() {
							outputSource.RunReturns(disaster)
						})

						It("invokes the delegate's output complete callback", func() {
							Ω(fakeOutputCompleteCallback.CallCallCount()).Should(Equal(1))
							cbErr, cbSource := fakeOutputCompleteCallback.CallArgsForCall(0)
							Ω(cbErr).Should(Equal(disaster))
							Ω(cbSource).Should(Equal(outputSource))
						})

						It("invokes the delegate's finish callback", func() {
							Ω(fakeFinishCallback.CallCallCount()).Should(Equal(1))
							cbErr, _ := fakeFinishCallback.CallArgsForCall(0)
							Ω(cbErr).Should(MatchError(ContainSubstring(disaster.Error())))
						})
					})
				})

				Context("when the output should perform on failure", func() {
					BeforeEach(func() {
						outputPlan.On = atc.OutputConditions{atc.OutputConditionFailure}
					})

					It("does not execute the output", func() {
						Ω(outputSource.RunCallCount()).Should(BeZero())
					})
				})

				Context("when the output should perform on success or failure", func() {
					BeforeEach(func() {
						outputPlan.On = atc.OutputConditions{atc.OutputConditionSuccess, atc.OutputConditionFailure}
					})

					It("executes the output", func() {
						Ω(outputSource.RunCallCount()).Should(Equal(1))
					})

					Context("when the output succeeds", func() {
						BeforeEach(func() {
							outputSource.RunReturns(nil)
						})

						It("invokes the delegate's output complete callback", func() {
							Ω(fakeOutputCompleteCallback.CallCallCount()).Should(Equal(1))
							cbErr, cbSource := fakeOutputCompleteCallback.CallArgsForCall(0)
							Ω(cbErr).ShouldNot(HaveOccurred())
							Ω(cbSource).Should(Equal(outputSource))
						})
					})

					Context("when the output fails", func() {
						disaster := errors.New("oh no!")

						BeforeEach(func() {
							outputSource.RunReturns(disaster)
						})

						It("invokes the delegate's output complete callback", func() {
							Ω(fakeOutputCompleteCallback.CallCallCount()).Should(Equal(1))
							cbErr, cbSource := fakeOutputCompleteCallback.CallArgsForCall(0)
							Ω(cbErr).Should(Equal(disaster))
							Ω(cbSource).Should(Equal(outputSource))
						})

						It("invokes the delegate's finish callback", func() {
							Ω(fakeFinishCallback.CallCallCount()).Should(Equal(1))
							cbErr, _ := fakeFinishCallback.CallArgsForCall(0)
							Ω(cbErr).Should(MatchError(ContainSubstring(disaster.Error())))
						})
					})
				})

				Context("when the output has empty conditions", func() {
					BeforeEach(func() {
						outputPlan.On = atc.OutputConditions{}
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
						outputPlan.On = atc.OutputConditions{atc.OutputConditionSuccess}
					})

					It("does not execute the output", func() {
						Ω(outputSource.RunCallCount()).Should(BeZero())
					})
				})

				Context("when the output should perform on failure", func() {
					BeforeEach(func() {
						outputPlan.On = atc.OutputConditions{atc.OutputConditionFailure}
					})

					It("executes the output", func() {
						Ω(outputSource.RunCallCount()).Should(Equal(1))
					})

					Context("when the output succeeds", func() {
						BeforeEach(func() {
							outputSource.RunReturns(nil)
						})

						It("invokes the delegate's output complete callback", func() {
							Ω(fakeOutputCompleteCallback.CallCallCount()).Should(Equal(1))
							cbErr, cbSource := fakeOutputCompleteCallback.CallArgsForCall(0)
							Ω(cbErr).ShouldNot(HaveOccurred())
							Ω(cbSource).Should(Equal(outputSource))
						})
					})

					Context("when the output fails", func() {
						disaster := errors.New("oh no!")

						BeforeEach(func() {
							outputSource.RunReturns(disaster)
						})

						It("invokes the delegate's output complete callback", func() {
							Ω(fakeOutputCompleteCallback.CallCallCount()).Should(Equal(1))
							cbErr, cbSource := fakeOutputCompleteCallback.CallArgsForCall(0)
							Ω(cbErr).Should(Equal(disaster))
							Ω(cbSource).Should(Equal(outputSource))
						})

						It("invokes the delegate's finish callback", func() {
							Ω(fakeFinishCallback.CallCallCount()).Should(Equal(1))
							cbErr, _ := fakeFinishCallback.CallArgsForCall(0)
							Ω(cbErr).Should(MatchError(ContainSubstring(disaster.Error())))
						})
					})
				})

				Context("when the output should perform on success or failure", func() {
					BeforeEach(func() {
						outputPlan.On = atc.OutputConditions{atc.OutputConditionSuccess, atc.OutputConditionFailure}
					})

					It("executes the output", func() {
						Ω(outputSource.RunCallCount()).Should(Equal(1))
					})

					Context("when the output succeeds", func() {
						BeforeEach(func() {
							outputSource.RunReturns(nil)
						})

						It("invokes the delegate's output complete callback", func() {
							Ω(fakeOutputCompleteCallback.CallCallCount()).Should(Equal(1))
							cbErr, cbSource := fakeOutputCompleteCallback.CallArgsForCall(0)
							Ω(cbErr).ShouldNot(HaveOccurred())
							Ω(cbSource).Should(Equal(outputSource))
						})
					})

					Context("when the output fails", func() {
						disaster := errors.New("oh no!")

						BeforeEach(func() {
							outputSource.RunReturns(disaster)
						})

						It("invokes the delegate's output complete callback", func() {
							Ω(fakeOutputCompleteCallback.CallCallCount()).Should(Equal(1))
							cbErr, cbSource := fakeOutputCompleteCallback.CallArgsForCall(0)
							Ω(cbErr).Should(Equal(disaster))
							Ω(cbSource).Should(Equal(outputSource))
						})

						It("invokes the delegate's finish callback", func() {
							Ω(fakeFinishCallback.CallCallCount()).Should(Equal(1))
							cbErr, _ := fakeFinishCallback.CallArgsForCall(0)
							Ω(cbErr).Should(MatchError(ContainSubstring(disaster.Error())))
						})
					})
				})

				Context("when the output has empty conditions", func() {
					BeforeEach(func() {
						outputPlan.On = atc.OutputConditions{}
					})

					It("does not execute the output", func() {
						Ω(outputSource.RunCallCount()).Should(BeZero())
					})
				})
			})

			Context("when no build is configured", func() {
				BeforeEach(func() {
					buildConfig = nil
					buildConfigPath = ""
				})

				It("executes the output", func() {
					Ω(outputSource.RunCallCount()).Should(Equal(1))
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

			It("invokes the delegate's input complete callback", func() {
				Ω(fakeInputCompleteCallback.CallCallCount()).Should(Equal(1))
				cbErr, cbSource := fakeInputCompleteCallback.CallArgsForCall(0)
				Ω(cbErr).Should(Equal(disaster))
				Ω(cbSource).Should(Equal(inputSource))
			})

			It("invokes the delegate's finish callback", func() {
				Ω(fakeFinishCallback.CallCallCount()).Should(Equal(1))
				cbErr, _ := fakeFinishCallback.CallArgsForCall(0)
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
