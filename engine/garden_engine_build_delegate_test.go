package engine_test

import (
	"errors"
	"time"

	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
	. "github.com/concourse/atc/engine"
	"github.com/concourse/atc/engine/fakes"
	"github.com/concourse/atc/event"
	"github.com/concourse/atc/exec"
	execfakes "github.com/concourse/atc/exec/fakes"
	"github.com/pivotal-golang/lager/lagertest"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("BuildDelegate", func() {
	var (
		fakeDB  *fakes.FakeEngineDB
		factory BuildDelegateFactory

		buildID int

		delegate BuildDelegate

		logger *lagertest.TestLogger
	)

	BeforeEach(func() {
		fakeDB = new(fakes.FakeEngineDB)
		factory = NewBuildDelegateFactory(fakeDB)

		buildID = 42
		delegate = factory.Delegate(buildID)

		logger = lagertest.NewTestLogger("test")
	})

	Describe("Start", func() {
		BeforeEach(func() {
			delegate.Start(logger)
		})

		It("saves the build's start time", func() {
			Ω(fakeDB.SaveBuildStartTimeCallCount()).Should(Equal(1))

			buildID, startTime := fakeDB.SaveBuildStartTimeArgsForCall(0)
			Ω(buildID).Should(Equal(42))
			Ω(startTime).Should(BeTemporally("~", time.Now(), time.Second))
		})

		It("saves the build's status as 'started'", func() {
			Ω(fakeDB.SaveBuildStatusCallCount()).Should(Equal(1))

			buildID, status := fakeDB.SaveBuildStatusArgsForCall(0)
			Ω(buildID).Should(Equal(42))
			Ω(status).Should(Equal(db.StatusStarted))
		})

		It("saves a start event and a 'started' status event", func() {
			Ω(fakeDB.SaveBuildStartTimeCallCount()).Should(Equal(1))

			_, startTime := fakeDB.SaveBuildStartTimeArgsForCall(0)

			Ω(fakeDB.SaveBuildEventCallCount()).Should(Equal(2))

			buildID, savedEvent := fakeDB.SaveBuildEventArgsForCall(0)
			Ω(buildID).Should(Equal(42))
			Ω(savedEvent).Should(Equal(event.Start{Time: startTime.Unix()}))

			buildID, savedEvent = fakeDB.SaveBuildEventArgsForCall(1)
			Ω(buildID).Should(Equal(42))
			Ω(savedEvent).Should(Equal(event.Status{
				Status: atc.StatusStarted,
				Time:   startTime.Unix(),
			}))
		})
	})

	Describe("InputCompleted", func() {
		var (
			inputPlan atc.InputPlan

			callback exec.CompleteCallback

			cbErr    error
			cbSource *execfakes.FakeArtifactSource
		)

		BeforeEach(func() {
			inputPlan = atc.InputPlan{
				Name:     "some-input",
				Resource: "some-input-resource",
				Type:     "some-type",
				Version:  atc.Version{"some": "version"},
				Source:   atc.Source{"some": "source"},
				Params:   atc.Params{"some": "params"},
			}

			callback = delegate.InputCompleted(logger, inputPlan)

			cbErr = nil
			cbSource = new(execfakes.FakeArtifactSource)
		})

		JustBeforeEach(func() {
			callback.Call(cbErr, cbSource)
		})

		Describe("success", func() {
			var versionInfo exec.VersionInfo

			BeforeEach(func() {
				cbErr = nil

				versionInfo = exec.VersionInfo{
					Version:  atc.Version{"result": "version"},
					Metadata: []atc.MetadataField{{"result", "metadata"}},
				}

				cbSource.ResultStub = versionInfoResult(versionInfo)
			})

			It("saves the build's input", func() {
				Ω(fakeDB.SaveBuildInputCallCount()).Should(Equal(1))

				buildID, savedInput := fakeDB.SaveBuildInputArgsForCall(0)
				Ω(buildID).Should(Equal(42))
				Ω(savedInput).Should(Equal(db.BuildInput{
					Name: "some-input",
					VersionedResource: db.VersionedResource{
						Resource: "some-input-resource",
						Type:     "some-type",
						Source:   db.Source{"some": "source"},
						Version:  db.Version{"result": "version"},
						Metadata: []db.MetadataField{{"result", "metadata"}},
					},
				}))
			})

			It("saves an input event", func() {
				Ω(fakeDB.SaveBuildEventCallCount()).Should(Equal(1))

				buildID, savedEvent := fakeDB.SaveBuildEventArgsForCall(0)
				Ω(buildID).Should(Equal(42))
				Ω(savedEvent).Should(Equal(event.Input{
					Plan:            inputPlan,
					FetchedVersion:  versionInfo.Version,
					FetchedMetadata: versionInfo.Metadata,
				}))
			})

			Context("when the resource only occurs as an input", func() {
				Describe("Finish", func() {
					var (
						finishCallback exec.CompleteCallback

						finishCBErr    error
						finishCBSource *execfakes.FakeArtifactSource
					)

					BeforeEach(func() {
						finishCallback = delegate.Finish(logger)

						finishCBErr = nil
						finishCBSource = new(execfakes.FakeArtifactSource)
					})

					JustBeforeEach(func() {
						finishCallback.Call(finishCBErr, finishCBSource)
					})

					Context("with success", func() {
						BeforeEach(func() {
							finishCBErr = nil
						})

						It("saves the input as an implicit output", func() {
							Ω(fakeDB.SaveBuildOutputCallCount()).Should(Equal(1))

							buildID, savedOutput := fakeDB.SaveBuildOutputArgsForCall(0)
							Ω(buildID).Should(Equal(42))
							Ω(savedOutput).Should(Equal(db.VersionedResource{
								Resource: "some-input-resource",
								Type:     "some-type",
								Source:   db.Source{"some": "source"},
								Version:  db.Version{"result": "version"},
								Metadata: []db.MetadataField{{"result", "metadata"}},
							}))
						})
					})

					Context("with failure", func() {
						disaster := errors.New("nope")

						BeforeEach(func() {
							finishCBErr = disaster
						})

						It("does not save the input as an implicit output", func() {
							Ω(fakeDB.SaveBuildOutputCallCount()).Should(BeZero())
						})
					})
				})
			})

			Context("when the same resource occurs as an explicit output", func() {
				var (
					outputPlan atc.OutputPlan

					outputCallback exec.CompleteCallback

					outputCBErr    error
					outputCBSource *execfakes.FakeArtifactSource
				)

				BeforeEach(func() {
					outputPlan = atc.OutputPlan{
						Name:   "some-input-resource",
						Type:   "some-type",
						On:     atc.OutputConditions{atc.OutputConditionSuccess},
						Source: atc.Source{"some": "source"},
						Params: atc.Params{"some": "output-params"},
					}

					outputCallback = delegate.OutputCompleted(logger, outputPlan)

					outputCBErr = nil
					outputCBSource = new(execfakes.FakeArtifactSource)
					outputCBSource.ResultStub = versionInfoResult(exec.VersionInfo{
						Version:  atc.Version{"explicit": "version"},
						Metadata: []atc.MetadataField{{"explicit", "metadata"}},
					})
				})

				JustBeforeEach(func() {
					outputCallback.Call(outputCBErr, outputCBSource)
				})

				Describe("Finish", func() {
					var (
						finishCallback exec.CompleteCallback

						finishCBErr    error
						finishCBSource *execfakes.FakeArtifactSource
					)

					BeforeEach(func() {
						finishCallback = delegate.Finish(logger)

						finishCBErr = nil
						finishCBSource = new(execfakes.FakeArtifactSource)
					})

					JustBeforeEach(func() {
						finishCallback.Call(finishCBErr, finishCBSource)
					})

					It("only saves the explicit output", func() {
						Ω(fakeDB.SaveBuildOutputCallCount()).Should(Equal(1))

						buildID, savedOutput := fakeDB.SaveBuildOutputArgsForCall(0)
						Ω(buildID).Should(Equal(42))
						Ω(savedOutput).Should(Equal(db.VersionedResource{
							Resource: "some-input-resource",
							Type:     "some-type",
							Source:   db.Source{"some": "source"},
							Version:  db.Version{"explicit": "version"},
							Metadata: []db.MetadataField{{"explicit", "metadata"}},
						}))
					})
				})
			})
		})

		Describe("failure", func() {
			BeforeEach(func() {
				cbErr = errors.New("nope")
			})

			It("does not save the build's input", func() {
				Ω(fakeDB.SaveBuildInputCallCount()).Should(BeZero())
			})

			It("saves an error event", func() {
				Ω(fakeDB.SaveBuildEventCallCount()).Should(Equal(1))

				buildID, savedEvent := fakeDB.SaveBuildEventArgsForCall(0)
				Ω(buildID).Should(Equal(42))
				Ω(savedEvent).Should(Equal(event.Error{
					Origin: event.Origin{
						Type: event.OriginTypeInput,
						Name: "some-input",
					},
					Message: "nope",
				}))
			})
		})
	})

	Describe("ExecutionCompleted", func() {
		var (
			callback exec.CompleteCallback

			cbErr    error
			cbSource *execfakes.FakeArtifactSource
		)

		BeforeEach(func() {
			callback = delegate.ExecutionCompleted(logger)

			cbErr = nil
			cbSource = new(execfakes.FakeArtifactSource)
		})

		JustBeforeEach(func() {
			callback.Call(cbErr, cbSource)
		})

		Describe("success", func() {
			BeforeEach(func() {
				cbErr = nil
			})

			Context("with a successful result", func() {
				BeforeEach(func() {
					cbSource.ResultStub = exitStatusResult(0)
				})

				It("saves a finish event", func() {
					Ω(fakeDB.SaveBuildEventCallCount()).Should(Equal(1))

					buildID, savedEvent := fakeDB.SaveBuildEventArgsForCall(0)
					Ω(buildID).Should(Equal(42))
					Ω(savedEvent).Should(BeAssignableToTypeOf(event.Finish{}))
					Ω(savedEvent.(event.Finish).ExitStatus).Should(Equal(0))
					Ω(savedEvent.(event.Finish).Time).Should(BeNumerically("<=", time.Now().Unix(), 1))
				})

				Describe("Finish", func() {
					var (
						finishCallback exec.CompleteCallback

						finishCBErr    error
						finishCBSource *execfakes.FakeArtifactSource
					)

					BeforeEach(func() {
						finishCallback = delegate.Finish(logger)

						finishCBErr = nil
						finishCBSource = new(execfakes.FakeArtifactSource)
					})

					JustBeforeEach(func() {
						finishCallback.Call(finishCBErr, finishCBSource)
					})

					Context("with success", func() {
						BeforeEach(func() {
							finishCBErr = nil
						})

						It("saves status as 'succeeded'", func() {
							Ω(fakeDB.SaveBuildStatusCallCount()).Should(Equal(1))

							buildID, savedStatus := fakeDB.SaveBuildStatusArgsForCall(0)
							Ω(buildID).Should(Equal(42))
							Ω(savedStatus).Should(Equal(db.StatusSucceeded))
						})
					})

					Context("with failure", func() {
						disaster := errors.New("nope")

						BeforeEach(func() {
							finishCBErr = disaster
						})

						It("saves status as 'errored'", func() {
							Ω(fakeDB.SaveBuildStatusCallCount()).Should(Equal(1))

							buildID, savedStatus := fakeDB.SaveBuildStatusArgsForCall(0)
							Ω(buildID).Should(Equal(42))
							Ω(savedStatus).Should(Equal(db.StatusErrored))
						})
					})
				})
			})

			Context("with a failed result", func() {
				BeforeEach(func() {
					cbSource.ResultStub = exitStatusResult(1)
				})

				It("saves a finish event", func() {
					Ω(fakeDB.SaveBuildEventCallCount()).Should(Equal(1))

					buildID, savedEvent := fakeDB.SaveBuildEventArgsForCall(0)
					Ω(buildID).Should(Equal(42))
					Ω(savedEvent).Should(BeAssignableToTypeOf(event.Finish{}))
					Ω(savedEvent.(event.Finish).ExitStatus).Should(Equal(1))
					Ω(savedEvent.(event.Finish).Time).Should(BeNumerically("<=", time.Now().Unix(), 1))
				})

				Describe("Finish", func() {
					var (
						finishCallback exec.CompleteCallback

						finishCBErr    error
						finishCBSource *execfakes.FakeArtifactSource
					)

					BeforeEach(func() {
						finishCallback = delegate.Finish(logger)

						finishCBErr = nil
						finishCBSource = new(execfakes.FakeArtifactSource)
					})

					JustBeforeEach(func() {
						finishCallback.Call(finishCBErr, finishCBSource)
					})

					Context("with success", func() {
						BeforeEach(func() {
							finishCBErr = nil
						})

						It("saves status as 'failed'", func() {
							Ω(fakeDB.SaveBuildStatusCallCount()).Should(Equal(1))

							buildID, savedStatus := fakeDB.SaveBuildStatusArgsForCall(0)
							Ω(buildID).Should(Equal(42))
							Ω(savedStatus).Should(Equal(db.StatusFailed))
						})
					})

					Context("with failure", func() {
						disaster := errors.New("nope")

						BeforeEach(func() {
							finishCBErr = disaster
						})

						It("saves status as 'errored'", func() {
							Ω(fakeDB.SaveBuildStatusCallCount()).Should(Equal(1))

							buildID, savedStatus := fakeDB.SaveBuildStatusArgsForCall(0)
							Ω(buildID).Should(Equal(42))
							Ω(savedStatus).Should(Equal(db.StatusErrored))
						})
					})
				})
			})
		})

		Describe("failure", func() {
			BeforeEach(func() {
				cbErr = errors.New("nope")
			})

			It("does not save the build's input", func() {
				Ω(fakeDB.SaveBuildInputCallCount()).Should(BeZero())
			})

			It("saves an error event", func() {
				Ω(fakeDB.SaveBuildEventCallCount()).Should(Equal(1))

				buildID, savedEvent := fakeDB.SaveBuildEventArgsForCall(0)
				Ω(buildID).Should(Equal(42))
				Ω(savedEvent).Should(Equal(event.Error{
					Message: "nope",
				}))
			})
		})
	})

	Describe("OutputCompleted", func() {
		var (
			outputPlan atc.OutputPlan

			callback exec.CompleteCallback

			cbErr    error
			cbSource *execfakes.FakeArtifactSource
		)

		BeforeEach(func() {
			outputPlan = atc.OutputPlan{
				Name:   "some-output-resource",
				Type:   "some-type",
				Source: atc.Source{"some": "source"},
				Params: atc.Params{"some": "params"},
			}

			callback = delegate.OutputCompleted(logger, outputPlan)

			cbErr = nil
			cbSource = new(execfakes.FakeArtifactSource)
		})

		JustBeforeEach(func() {
			callback.Call(cbErr, cbSource)
		})

		Describe("success", func() {
			var versionInfo exec.VersionInfo

			BeforeEach(func() {
				cbErr = nil

				versionInfo = exec.VersionInfo{
					Version:  atc.Version{"result": "version"},
					Metadata: []atc.MetadataField{{"result", "metadata"}},
				}

				cbSource.ResultStub = versionInfoResult(versionInfo)
			})

			It("saves the build's output", func() {
				Ω(fakeDB.SaveBuildOutputCallCount()).Should(Equal(1))

				buildID, savedOutput := fakeDB.SaveBuildOutputArgsForCall(0)
				Ω(buildID).Should(Equal(42))
				Ω(savedOutput).Should(Equal(db.VersionedResource{
					Resource: "some-output-resource",
					Type:     "some-type",
					Source:   db.Source{"some": "source"},
					Version:  db.Version{"result": "version"},
					Metadata: []db.MetadataField{{"result", "metadata"}},
				}))
			})

			It("saves an output event", func() {
				Ω(fakeDB.SaveBuildEventCallCount()).Should(Equal(1))

				buildID, savedEvent := fakeDB.SaveBuildEventArgsForCall(0)
				Ω(buildID).Should(Equal(42))
				Ω(savedEvent).Should(Equal(event.Output{
					Plan:            outputPlan,
					CreatedVersion:  versionInfo.Version,
					CreatedMetadata: versionInfo.Metadata,
				}))
			})
		})

		Describe("failure", func() {
			BeforeEach(func() {
				cbErr = errors.New("nope")
			})

			It("does not save the build's input", func() {
				Ω(fakeDB.SaveBuildInputCallCount()).Should(BeZero())
			})

			It("saves an error event", func() {
				Ω(fakeDB.SaveBuildEventCallCount()).Should(Equal(1))

				buildID, savedEvent := fakeDB.SaveBuildEventArgsForCall(0)
				Ω(buildID).Should(Equal(42))
				Ω(savedEvent).Should(Equal(event.Error{
					Origin: event.Origin{
						Type: event.OriginTypeOutput,
						Name: "some-output-resource",
					},
					Message: "nope",
				}))
			})
		})
	})

	Describe("Aborted", func() {
		JustBeforeEach(func() {
			delegate.Aborted(logger)
		})

		Describe("Finish", func() {
			var (
				finishCallback exec.CompleteCallback

				finishCBErr    error
				finishCBSource *execfakes.FakeArtifactSource
			)

			BeforeEach(func() {
				finishCallback = delegate.Finish(logger)

				finishCBErr = nil
				finishCBSource = new(execfakes.FakeArtifactSource)
			})

			JustBeforeEach(func() {
				finishCallback.Call(finishCBErr, finishCBSource)
			})

			Context("with success", func() {
				BeforeEach(func() {
					finishCBErr = nil
				})

				It("saves status as 'aborted'", func() {
					Ω(fakeDB.SaveBuildStatusCallCount()).Should(Equal(1))

					buildID, savedStatus := fakeDB.SaveBuildStatusArgsForCall(0)
					Ω(buildID).Should(Equal(42))
					Ω(savedStatus).Should(Equal(db.StatusAborted))
				})
			})

			Context("with failure", func() {
				disaster := errors.New("nope")

				BeforeEach(func() {
					finishCBErr = disaster
				})

				It("saves status as 'aborted'", func() {
					Ω(fakeDB.SaveBuildStatusCallCount()).Should(Equal(1))

					buildID, savedStatus := fakeDB.SaveBuildStatusArgsForCall(0)
					Ω(buildID).Should(Equal(42))
					Ω(savedStatus).Should(Equal(db.StatusAborted))
				})
			})
		})
	})
})

func versionInfoResult(result exec.VersionInfo) func(dest interface{}) bool {
	return func(dest interface{}) bool {
		switch x := dest.(type) {
		case *exec.VersionInfo:
			*x = result
			return true

		default:
			return false
		}
	}
}

func exitStatusResult(result exec.ExitStatus) func(dest interface{}) bool {
	return func(dest interface{}) bool {
		switch x := dest.(type) {
		case *exec.ExitStatus:
			*x = result
			return true

		case *exec.Success:
			*x = result == 0
			return true

		default:
			return false
		}
	}
}
