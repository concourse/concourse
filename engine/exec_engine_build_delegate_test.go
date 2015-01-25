package engine_test

import (
	"errors"
	"io"
	"time"

	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
	. "github.com/concourse/atc/engine"
	"github.com/concourse/atc/engine/fakes"
	"github.com/concourse/atc/event"
	"github.com/concourse/atc/exec"
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

	Describe("InputDelegate", func() {
		var (
			inputPlan atc.InputPlan

			inputDelegate exec.GetDelegate
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

			inputDelegate = delegate.InputDelegate(logger, inputPlan)
		})

		Describe("Completed", func() {
			var versionInfo exec.VersionInfo

			BeforeEach(func() {
				versionInfo = exec.VersionInfo{
					Version:  atc.Version{"result": "version"},
					Metadata: []atc.MetadataField{{"result", "metadata"}},
				}
			})

			JustBeforeEach(func() {
				inputDelegate.Completed(versionInfo)
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
					var finishErr error

					JustBeforeEach(func() {
						delegate.Finish(logger, finishErr)
					})

					Context("with success", func() {
						BeforeEach(func() {
							finishErr = nil
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
							finishErr = disaster
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

					outputDelegate exec.PutDelegate
				)

				BeforeEach(func() {
					outputPlan = atc.OutputPlan{
						Name:   "some-input-resource",
						Type:   "some-type",
						On:     atc.OutputConditions{atc.OutputConditionSuccess},
						Source: atc.Source{"some": "source"},
						Params: atc.Params{"some": "output-params"},
					}

					outputDelegate = delegate.OutputDelegate(logger, outputPlan)
				})

				JustBeforeEach(func() {
					outputDelegate.Completed(exec.VersionInfo{
						Version:  atc.Version{"explicit": "version"},
						Metadata: []atc.MetadataField{{"explicit", "metadata"}},
					})
				})

				Describe("Finish", func() {
					var finishErr error

					BeforeEach(func() {
						finishErr = nil
					})

					JustBeforeEach(func() {
						delegate.Finish(logger, finishErr)
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

		Describe("Failed", func() {
			JustBeforeEach(func() {
				inputDelegate.Failed(errors.New("nope"))
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

		Describe("Stdout", func() {
			var writer io.Writer

			BeforeEach(func() {
				writer = inputDelegate.Stdout()
			})

			It("saves log events with the input's origin", func() {
				_, err := writer.Write([]byte("some stdout"))
				Ω(err).ShouldNot(HaveOccurred())

				Ω(fakeDB.SaveBuildEventCallCount()).Should(Equal(1))

				savedBuildID, savedEvent := fakeDB.SaveBuildEventArgsForCall(0)
				Ω(savedBuildID).Should(Equal(buildID))
				Ω(savedEvent).Should(Equal(event.Log{
					Origin: event.Origin{
						Type: event.OriginTypeInput,
						Name: "some-input",
					},
					Payload: "some stdout",
				}))
			})
		})

		Describe("Stderr", func() {
			var writer io.Writer

			BeforeEach(func() {
				writer = inputDelegate.Stderr()
			})

			It("saves log events with the input's origin", func() {
				_, err := writer.Write([]byte("some stderr"))
				Ω(err).ShouldNot(HaveOccurred())

				Ω(fakeDB.SaveBuildEventCallCount()).Should(Equal(1))

				savedBuildID, savedEvent := fakeDB.SaveBuildEventArgsForCall(0)
				Ω(savedBuildID).Should(Equal(buildID))
				Ω(savedEvent).Should(Equal(event.Log{
					Origin: event.Origin{
						Type: event.OriginTypeInput,
						Name: "some-input",
					},
					Payload: "some stderr",
				}))
			})
		})
	})

	Describe("ExecutionDelegate", func() {
		var (
			executionDelegate exec.ExecuteDelegate
		)

		BeforeEach(func() {
			executionDelegate = delegate.ExecutionDelegate(logger)
		})

		Describe("Initializing", func() {
			var buildConfig atc.BuildConfig

			BeforeEach(func() {
				buildConfig = atc.BuildConfig{
					Run: atc.BuildRunConfig{
						Path: "ls",
					},
				}
			})

			JustBeforeEach(func() {
				executionDelegate.Initializing(buildConfig)
			})

			It("saves an initialize event", func() {
				Ω(fakeDB.SaveBuildEventCallCount()).Should(Equal(1))

				buildID, savedEvent := fakeDB.SaveBuildEventArgsForCall(0)
				Ω(buildID).Should(Equal(42))
				Ω(savedEvent).Should(Equal(event.Initialize{
					BuildConfig: buildConfig,
				}))
			})
		})

		Describe("Started", func() {
			JustBeforeEach(func() {
				executionDelegate.Started()
			})

			It("saves a start event", func() {
				Ω(fakeDB.SaveBuildEventCallCount()).Should(Equal(1))

				buildID, savedEvent := fakeDB.SaveBuildEventArgsForCall(0)
				Ω(buildID).Should(Equal(42))
				Ω(savedEvent).Should(BeAssignableToTypeOf(event.Start{}))
				Ω(savedEvent.(event.Start).Time).Should(BeNumerically("~", time.Now().Unix(), 1))
			})
		})

		Describe("Finished", func() {
			var exitStatus exec.ExitStatus

			BeforeEach(func() {
				exitStatus = 0
			})

			JustBeforeEach(func() {
				executionDelegate.Finished(exitStatus)
			})

			Context("with a successful result", func() {
				BeforeEach(func() {
					exitStatus = 0
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
					var finishErr error

					BeforeEach(func() {
						finishErr = nil
					})

					JustBeforeEach(func() {
						delegate.Finish(logger, finishErr)
					})

					Context("with success", func() {
						BeforeEach(func() {
							finishErr = nil
						})

						It("finishes with status 'succeeded'", func() {
							Ω(fakeDB.FinishBuildCallCount()).Should(Equal(1))

							buildID, savedStatus := fakeDB.FinishBuildArgsForCall(0)
							Ω(buildID).Should(Equal(42))
							Ω(savedStatus).Should(Equal(db.StatusSucceeded))
						})
					})

					Context("with failure", func() {
						disaster := errors.New("nope")

						BeforeEach(func() {
							finishErr = disaster
						})

						It("finishes with status 'errored'", func() {
							Ω(fakeDB.FinishBuildCallCount()).Should(Equal(1))

							buildID, savedStatus := fakeDB.FinishBuildArgsForCall(0)
							Ω(buildID).Should(Equal(42))
							Ω(savedStatus).Should(Equal(db.StatusErrored))
						})
					})
				})
			})

			Context("with a failed result", func() {
				BeforeEach(func() {
					exitStatus = 1
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
					var finishErr error

					BeforeEach(func() {
						finishErr = nil
					})

					JustBeforeEach(func() {
						delegate.Finish(logger, finishErr)
					})

					Context("with success", func() {
						BeforeEach(func() {
							finishErr = nil
						})

						It("finishes with status 'failed'", func() {
							Ω(fakeDB.FinishBuildCallCount()).Should(Equal(1))

							buildID, savedStatus := fakeDB.FinishBuildArgsForCall(0)
							Ω(buildID).Should(Equal(42))
							Ω(savedStatus).Should(Equal(db.StatusFailed))
						})
					})

					Context("with failure", func() {
						disaster := errors.New("nope")

						BeforeEach(func() {
							finishErr = disaster
						})

						It("finishes with status 'errored'", func() {
							Ω(fakeDB.FinishBuildCallCount()).Should(Equal(1))

							buildID, savedStatus := fakeDB.FinishBuildArgsForCall(0)
							Ω(buildID).Should(Equal(42))
							Ω(savedStatus).Should(Equal(db.StatusErrored))
						})
					})
				})
			})
		})

		Describe("Failed", func() {
			JustBeforeEach(func() {
				executionDelegate.Failed(errors.New("nope"))
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

		Describe("Stdout", func() {
			var writer io.Writer

			BeforeEach(func() {
				writer = executionDelegate.Stdout()
			})

			It("saves log events with the correct origin", func() {
				_, err := writer.Write([]byte("some stdout"))
				Ω(err).ShouldNot(HaveOccurred())

				Ω(fakeDB.SaveBuildEventCallCount()).Should(Equal(1))

				savedBuildID, savedEvent := fakeDB.SaveBuildEventArgsForCall(0)
				Ω(savedBuildID).Should(Equal(buildID))
				Ω(savedEvent).Should(Equal(event.Log{
					Origin: event.Origin{
						Type: event.OriginTypeRun,
						Name: "stdout",
					},
					Payload: "some stdout",
				}))
			})
		})

		Describe("Stderr", func() {
			var writer io.Writer

			BeforeEach(func() {
				writer = executionDelegate.Stderr()
			})

			It("saves log events with the correct origin", func() {
				_, err := writer.Write([]byte("some stderr"))
				Ω(err).ShouldNot(HaveOccurred())

				Ω(fakeDB.SaveBuildEventCallCount()).Should(Equal(1))

				savedBuildID, savedEvent := fakeDB.SaveBuildEventArgsForCall(0)
				Ω(savedBuildID).Should(Equal(buildID))
				Ω(savedEvent).Should(Equal(event.Log{
					Origin: event.Origin{
						Type: event.OriginTypeRun,
						Name: "stderr",
					},
					Payload: "some stderr",
				}))
			})
		})
	})

	Describe("OutputDelegate", func() {
		var (
			outputPlan atc.OutputPlan

			outputDelegate exec.PutDelegate
		)

		BeforeEach(func() {
			outputPlan = atc.OutputPlan{
				Name:   "some-output-resource",
				Type:   "some-type",
				Source: atc.Source{"some": "source"},
				Params: atc.Params{"some": "params"},
			}

			outputDelegate = delegate.OutputDelegate(logger, outputPlan)
		})

		Describe("Completed", func() {
			var versionInfo exec.VersionInfo

			BeforeEach(func() {
				versionInfo = exec.VersionInfo{
					Version:  atc.Version{"result": "version"},
					Metadata: []atc.MetadataField{{"result", "metadata"}},
				}
			})

			JustBeforeEach(func() {
				outputDelegate.Completed(versionInfo)
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

		Describe("Failed", func() {
			JustBeforeEach(func() {
				outputDelegate.Failed(errors.New("nope"))
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

		Describe("Stdout", func() {
			var writer io.Writer

			BeforeEach(func() {
				writer = outputDelegate.Stdout()
			})

			It("saves log events with the output's origin", func() {
				_, err := writer.Write([]byte("some stdout"))
				Ω(err).ShouldNot(HaveOccurred())

				Ω(fakeDB.SaveBuildEventCallCount()).Should(Equal(1))

				savedBuildID, savedEvent := fakeDB.SaveBuildEventArgsForCall(0)
				Ω(savedBuildID).Should(Equal(buildID))
				Ω(savedEvent).Should(Equal(event.Log{
					Origin: event.Origin{
						Type: event.OriginTypeOutput,
						Name: "some-output-resource",
					},
					Payload: "some stdout",
				}))
			})
		})

		Describe("Stderr", func() {
			var writer io.Writer

			BeforeEach(func() {
				writer = outputDelegate.Stderr()
			})

			It("saves log events with the output's origin", func() {
				_, err := writer.Write([]byte("some stderr"))
				Ω(err).ShouldNot(HaveOccurred())

				Ω(fakeDB.SaveBuildEventCallCount()).Should(Equal(1))

				savedBuildID, savedEvent := fakeDB.SaveBuildEventArgsForCall(0)
				Ω(savedBuildID).Should(Equal(buildID))
				Ω(savedEvent).Should(Equal(event.Log{
					Origin: event.Origin{
						Type: event.OriginTypeOutput,
						Name: "some-output-resource",
					},
					Payload: "some stderr",
				}))
			})
		})
	})

	Describe("Aborted", func() {
		JustBeforeEach(func() {
			delegate.Aborted(logger)
		})

		Describe("Finish", func() {
			var finishErr error

			BeforeEach(func() {
				finishErr = nil
			})

			JustBeforeEach(func() {
				delegate.Finish(logger, finishErr)
			})

			Context("with success", func() {
				BeforeEach(func() {
					finishErr = nil
				})

				It("finishes with status 'aborted'", func() {
					Ω(fakeDB.FinishBuildCallCount()).Should(Equal(1))

					buildID, savedStatus := fakeDB.FinishBuildArgsForCall(0)
					Ω(buildID).Should(Equal(42))
					Ω(savedStatus).Should(Equal(db.StatusAborted))
				})
			})

			Context("with failure", func() {
				disaster := errors.New("nope")

				BeforeEach(func() {
					finishErr = disaster
				})

				It("finishes with status 'aborted'", func() {
					Ω(fakeDB.FinishBuildCallCount()).Should(Equal(1))

					buildID, savedStatus := fakeDB.FinishBuildArgsForCall(0)
					Ω(buildID).Should(Equal(42))
					Ω(savedStatus).Should(Equal(db.StatusAborted))
				})
			})
		})
	})
})
