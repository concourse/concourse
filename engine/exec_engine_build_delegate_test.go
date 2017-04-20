package engine_test

import (
	"errors"
	"io"
	"time"

	"code.cloudfoundry.org/lager/lagertest"
	"github.com/concourse/atc"
	"github.com/concourse/atc/dbng"
	"github.com/concourse/atc/dbng/dbngfakes"
	. "github.com/concourse/atc/engine"
	"github.com/concourse/atc/event"
	"github.com/concourse/atc/exec"
	"github.com/concourse/atc/worker"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("BuildDelegate", func() {
	var (
		factory BuildDelegateFactory

		fakeBuild *dbngfakes.FakeBuild

		delegate BuildDelegate

		logger *lagertest.TestLogger

		originID event.OriginID
	)

	BeforeEach(func() {
		factory = NewBuildDelegateFactory()

		fakeBuild = new(dbngfakes.FakeBuild)
		delegate = factory.Delegate(fakeBuild)

		logger = lagertest.NewTestLogger("test")

		originID = event.OriginID("some-origin-id")
	})

	Describe("InputDelegate", func() {
		var (
			getPlan atc.GetPlan

			inputDelegate exec.GetDelegate
		)

		BeforeEach(func() {
			getPlan = atc.GetPlan{
				Name:     "some-input",
				Resource: "some-input-resource",
				Type:     "some-type",
				Version:  atc.Version{"some": "version"},
				Source:   atc.Source{"some": "source"},
				Params:   atc.Params{"some": "params"},
			}

			inputDelegate = delegate.InputDelegate(logger, getPlan, originID)
		})

		Describe("Initializing", func() {
			JustBeforeEach(func() {
				inputDelegate.Initializing()
			})

			It("saves an initializing event", func() {
				Expect(fakeBuild.SaveEventCallCount()).To(Equal(1))

				savedEvent := fakeBuild.SaveEventArgsForCall(0)
				Expect(savedEvent).To(Equal(event.InitializeGet{
					Origin: event.Origin{
						ID: originID,
					},
				}))
			})
		})

		Describe("Completed", func() {
			var versionInfo *exec.VersionInfo

			BeforeEach(func() {
				versionInfo = &exec.VersionInfo{
					Version:  atc.Version{"result": "version"},
					Metadata: []atc.MetadataField{{"result", "metadata"}},
				}
			})

			Context("when exit status is not 0", func() {
				JustBeforeEach(func() {
					inputDelegate.Completed(exec.ExitStatus(12), nil)
				})

				It("does not save the build's input", func() {
					Expect(fakeBuild.SaveInputCallCount()).To(Equal(0))
				})

				It("saves a finish-get event", func() {
					Expect(fakeBuild.SaveEventCallCount()).To(Equal(1))

					savedEvent := fakeBuild.SaveEventArgsForCall(0)
					Expect(savedEvent).To(Equal(event.FinishGet{
						Origin: event.Origin{
							ID: originID,
						},
						Plan: event.GetPlan{
							Name:     "some-input",
							Resource: "some-input-resource",
							Type:     "some-type",
							Version:  atc.Version{"some": "version"},
						},
						ExitStatus: 12,
					}))
				})
			})

			Context("when the version is null", func() {
				JustBeforeEach(func() {
					inputDelegate.Completed(exec.ExitStatus(12), nil)
				})

				It("does not save the build's input", func() {
					Expect(fakeBuild.SaveInputCallCount()).To(Equal(0))
				})

				It("saves a finish-get event", func() {
					Expect(fakeBuild.SaveEventCallCount()).To(Equal(1))

					savedEvent := fakeBuild.SaveEventArgsForCall(0)
					Expect(savedEvent).To(Equal(event.FinishGet{
						Origin: event.Origin{
							ID: originID,
						},
						Plan: event.GetPlan{
							Name:     "some-input",
							Resource: "some-input-resource",
							Type:     "some-type",
							Version:  atc.Version{"some": "version"},
						},
						ExitStatus:      12,
						FetchedVersion:  nil,
						FetchedMetadata: nil,
					}))
				})
			})

			Describe("Finish", func() {
				var (
					finishErr error
					aborted   bool
					succeeded exec.Success
				)

				Context("without error", func() {
					BeforeEach(func() {
						finishErr = nil
					})

					Context("when it was told it failed", func() {
						BeforeEach(func() {
							succeeded = false
							aborted = false
						})

						It("finishes with status 'failed'", func() {
							delegate.Finish(logger, finishErr, succeeded, aborted)

							Expect(fakeBuild.FinishCallCount()).To(Equal(1))

							savedStatus := fakeBuild.FinishArgsForCall(0)
							Expect(savedStatus).To(Equal(dbng.BuildStatusFailed))
						})
					})

					Context("when it was told it succeeded", func() {
						BeforeEach(func() {
							succeeded = true
						})

						It("finishes with status 'succeeded'", func() {
							delegate.Finish(logger, finishErr, succeeded, aborted)

							Expect(fakeBuild.FinishCallCount()).To(Equal(1))

							savedStatus := fakeBuild.FinishArgsForCall(0)
							Expect(savedStatus).To(Equal(dbng.BuildStatusSucceeded))
						})
					})
				})

				Context("when exit status is 0", func() {
					BeforeEach(func() {
						fakeBuild.SaveInputReturns(nil)
					})

					JustBeforeEach(func() {
						inputDelegate.Completed(exec.ExitStatus(0), versionInfo)
					})

					It("saves the build's input", func() {
						Expect(fakeBuild.SaveInputCallCount()).To(Equal(1))

						savedInput := fakeBuild.SaveInputArgsForCall(0)
						Expect(savedInput).To(Equal(dbng.BuildInput{
							Name: "some-input",
							VersionedResource: dbng.VersionedResource{
								Resource: "some-input-resource",
								Type:     "some-type",
								Version:  dbng.ResourceVersion{"result": "version"},
								Metadata: []dbng.ResourceMetadataField{{"result", "metadata"}},
							},
						}))
					})

					It("saves a finish-get event", func() {
						Expect(fakeBuild.SaveEventCallCount()).To(Equal(1))

						savedEvent := fakeBuild.SaveEventArgsForCall(0)
						Expect(savedEvent).To(Equal(event.FinishGet{
							Origin: event.Origin{
								ID: originID,
							},
							Plan: event.GetPlan{
								Name:     "some-input",
								Resource: "some-input-resource",
								Type:     "some-type",
								Version:  atc.Version{"some": "version"},
							},
							FetchedVersion:  versionInfo.Version,
							FetchedMetadata: []atc.MetadataField{{"result", "metadata"}},
						}))
					})

					Context("when the resource only occurs as an input", func() {
						Describe("Finish", func() {
							var (
								finishErr error
								aborted   bool
								succeeded exec.Success
							)

							Context("with success", func() {
								BeforeEach(func() {
									finishErr = nil
									succeeded = true
									aborted = false
								})

								It("saves the input as an implicit output", func() {
									delegate.Finish(logger, finishErr, succeeded, aborted)

									Expect(fakeBuild.SaveOutputCallCount()).To(Equal(1))

									savedOutput, explicit := fakeBuild.SaveOutputArgsForCall(0)
									Expect(savedOutput).To(Equal(dbng.VersionedResource{
										Resource: "some-input-resource",
										Type:     "some-type",
										Version:  dbng.ResourceVersion{"result": "version"},
										Metadata: []dbng.ResourceMetadataField{{"result", "metadata"}},
									}))

									Expect(explicit).To(BeFalse())
								})
							})

							Context("with failure", func() {
								disaster := errors.New("nope")

								BeforeEach(func() {
									finishErr = disaster
									succeeded = false
								})

								It("does not save the input as an implicit output", func() {
									delegate.Finish(logger, finishErr, succeeded, aborted)

									Expect(fakeBuild.SaveOutputCallCount()).To(BeZero())
								})
							})
						})
					})

					Context("when the same resource occurs as an explicit output", func() {
						var (
							putPlan atc.PutPlan

							outputDelegate exec.PutDelegate
						)

						BeforeEach(func() {
							putPlan = atc.PutPlan{
								Resource: "some-input-resource",
								Type:     "some-type",
								Source:   atc.Source{"some": "source"},
								Params:   atc.Params{"some": "output-params"},
							}

							outputDelegate = delegate.OutputDelegate(logger, putPlan, originID)
						})

						JustBeforeEach(func() {
							outputDelegate.Completed(exec.ExitStatus(0), &exec.VersionInfo{
								Version:  atc.Version{"explicit": "version"},
								Metadata: []atc.MetadataField{{"explicit", "metadata"}},
							})
						})

						Describe("Finish", func() {
							var (
								finishErr error
								succeeded exec.Success
							)

							BeforeEach(func() {
								finishErr = nil
								succeeded = true
							})

							It("only saves the explicit output", func() {
								delegate.Finish(logger, finishErr, succeeded, aborted)

								Expect(fakeBuild.SaveOutputCallCount()).To(Equal(1))

								savedOutput, explicit := fakeBuild.SaveOutputArgsForCall(0)
								Expect(savedOutput).To(Equal(dbng.VersionedResource{
									Resource: "some-input-resource",
									Type:     "some-type",
									Version:  dbng.ResourceVersion{"explicit": "version"},
									Metadata: []dbng.ResourceMetadataField{{"explicit", "metadata"}},
								}))

								Expect(explicit).To(BeTrue())
							})
						})
					})
				})
			})
		})

		Describe("Failed", func() {
			JustBeforeEach(func() {
				inputDelegate.Failed(errors.New("nope"))
			})

			It("does not save the build's input", func() {
				Expect(fakeBuild.SaveInputCallCount()).To(BeZero())
			})

			It("saves an error event", func() {
				Expect(fakeBuild.SaveEventCallCount()).To(Equal(1))

				savedEvent := fakeBuild.SaveEventArgsForCall(0)
				Expect(savedEvent).To(Equal(event.Error{
					Origin: event.Origin{
						ID: originID,
					},
					Message: "nope",
				}))
			})
		})

		Describe("ImageVersionDetermined", func() {
			var resourceCacheIdentifier worker.ResourceCacheIdentifier

			BeforeEach(func() {
				resourceCacheIdentifier = worker.ResourceCacheIdentifier{
					ResourceVersion: atc.Version{"ref": "asdf"},
					ResourceHash:    "our-super-sweet-resource-hash",
				}
			})

			It("calls through to the database", func() {
				fakeBuild.SaveImageResourceVersionReturns(nil)

				err := inputDelegate.ImageVersionDetermined(resourceCacheIdentifier)
				Expect(err).ToNot(HaveOccurred())

				Expect(fakeBuild.SaveImageResourceVersionCallCount()).To(Equal(1))
				actualPlanID, actualResourceVersion, actualResourceHash := fakeBuild.SaveImageResourceVersionArgsForCall(0)
				Expect(actualPlanID).To(Equal(atc.PlanID("some-origin-id")))
				Expect(actualResourceVersion).To(Equal(atc.Version{"ref": "asdf"}))
				Expect(actualResourceHash).To(Equal("our-super-sweet-resource-hash"))
			})

			It("propagates errors", func() {
				distaster := errors.New("sorry mate")
				fakeBuild.SaveImageResourceVersionReturns(distaster)

				err := inputDelegate.ImageVersionDetermined(resourceCacheIdentifier)
				Expect(err).To(Equal(distaster))
			})
		})

		Describe("Stdout", func() {
			var writer io.Writer

			BeforeEach(func() {
				writer = inputDelegate.Stdout()
			})

			It("saves log events with the input's origin", func() {
				_, err := writer.Write([]byte("some stdout"))
				Expect(err).NotTo(HaveOccurred())

				Expect(fakeBuild.SaveEventCallCount()).To(Equal(1))

				savedEvent := fakeBuild.SaveEventArgsForCall(0)
				Expect(savedEvent).To(Equal(event.Log{
					Origin: event.Origin{
						Source: event.OriginSourceStdout,
						ID:     originID,
					},
					Payload: "some stdout",
				}))
			})

			Context("when the DB errors", func() {
				disaster := errors.New("nope")

				BeforeEach(func() {
					fakeBuild.SaveEventReturns(disaster)
				})

				It("returns the error", func() {
					_, err := writer.Write([]byte("some stderr"))
					Expect(err).To(Equal(disaster))
				})
			})
		})

		Describe("Stderr", func() {
			var writer io.Writer

			BeforeEach(func() {
				writer = inputDelegate.Stderr()
			})

			It("saves log events with the input's origin", func() {
				_, err := writer.Write([]byte("some stderr"))
				Expect(err).NotTo(HaveOccurred())

				Expect(fakeBuild.SaveEventCallCount()).To(Equal(1))

				savedEvent := fakeBuild.SaveEventArgsForCall(0)
				Expect(savedEvent).To(Equal(event.Log{
					Origin: event.Origin{
						Source: event.OriginSourceStderr,
						ID:     originID,
					},
					Payload: "some stderr",
				}))
			})

			Context("when the DB errors", func() {
				disaster := errors.New("nope")

				BeforeEach(func() {
					fakeBuild.SaveEventReturns(disaster)
				})

				It("returns the error", func() {
					_, err := writer.Write([]byte("some stderr"))
					Expect(err).To(Equal(disaster))
				})
			})
		})
	})

	Describe("ExecutionDelegate", func() {
		var (
			taskPlan          atc.TaskPlan
			executionDelegate exec.TaskDelegate
		)

		BeforeEach(func() {
			taskPlan = atc.TaskPlan{
				Name:       "some-task",
				Privileged: true,
				ConfigPath: "/etc/concourse/config.yml",
			}

			executionDelegate = delegate.ExecutionDelegate(logger, taskPlan, originID)
		})

		Describe("Initializing", func() {
			var taskConfig atc.TaskConfig

			BeforeEach(func() {
				taskConfig = atc.TaskConfig{
					Run: atc.TaskRunConfig{
						Path: "ls",
						Dir:  "some/dir",
					},
				}
			})

			JustBeforeEach(func() {
				executionDelegate.Initializing(taskConfig)
			})

			It("saves an initialize event", func() {
				Expect(fakeBuild.SaveEventCallCount()).To(Equal(1))

				savedEvent := fakeBuild.SaveEventArgsForCall(0)
				Expect(savedEvent).To(Equal(event.InitializeTask{
					TaskConfig: event.TaskConfig{
						Run: event.TaskRunConfig{
							Path: "ls",
							Dir:  "some/dir",
						},
					},
					Origin: event.Origin{
						ID: originID,
					},
				}))
			})
		})

		Describe("Started", func() {
			JustBeforeEach(func() {
				executionDelegate.Started()
			})

			It("saves a start event", func() {
				Expect(fakeBuild.SaveEventCallCount()).To(Equal(1))

				savedEvent := fakeBuild.SaveEventArgsForCall(0)
				Expect(savedEvent).To(BeAssignableToTypeOf(event.StartTask{}))
				Expect(savedEvent.(event.StartTask).Time).To(BeNumerically("~", time.Now().Unix(), 1))
				Expect(savedEvent.(event.StartTask).Origin).To(Equal(event.Origin{
					ID: originID,
				}))

			})
		})

		Describe("Finished", func() {
			var exitStatus exec.ExitStatus

			JustBeforeEach(func() {
				executionDelegate.Finished(exitStatus)
			})

			Context("with a successful result", func() {
				BeforeEach(func() {
					exitStatus = 0
				})

				It("saves a finish event", func() {
					Expect(fakeBuild.SaveEventCallCount()).To(Equal(1))

					savedEvent := fakeBuild.SaveEventArgsForCall(0)
					Expect(savedEvent).To(BeAssignableToTypeOf(event.FinishTask{}))
					Expect(savedEvent.(event.FinishTask).ExitStatus).To(Equal(0))
					Expect(savedEvent.(event.FinishTask).Time).To(BeNumerically("<=", time.Now().Unix(), 1))
					Expect(savedEvent.(event.FinishTask).Origin).To(Equal(event.Origin{
						ID: originID,
					}))

				})

				Describe("Finish", func() {
					var (
						finishErr error
						aborted   bool
						succeeded exec.Success
					)

					Context("with success", func() {
						BeforeEach(func() {
							finishErr = nil
							succeeded = true
							aborted = false
						})

						It("finishes with status 'succeeded'", func() {
							delegate.Finish(logger, finishErr, succeeded, aborted)

							Expect(fakeBuild.FinishCallCount()).To(Equal(1))

							savedStatus := fakeBuild.FinishArgsForCall(0)
							Expect(savedStatus).To(Equal(dbng.BuildStatusSucceeded))
						})
					})

					Context("with failure", func() {
						disaster := errors.New("nope")

						BeforeEach(func() {
							finishErr = disaster
							succeeded = false
						})

						It("finishes with status 'errored'", func() {
							delegate.Finish(logger, finishErr, succeeded, aborted)

							Expect(fakeBuild.FinishCallCount()).To(Equal(1))

							savedStatus := fakeBuild.FinishArgsForCall(0)
							Expect(savedStatus).To(Equal(dbng.BuildStatusErrored))
						})
					})
				})
			})

			Context("with a failed result", func() {
				BeforeEach(func() {
					exitStatus = 1
				})

				It("saves a finish event", func() {
					Expect(fakeBuild.SaveEventCallCount()).To(Equal(1))

					savedEvent := fakeBuild.SaveEventArgsForCall(0)
					Expect(savedEvent).To(BeAssignableToTypeOf(event.FinishTask{}))
					Expect(savedEvent.(event.FinishTask).ExitStatus).To(Equal(1))
					Expect(savedEvent.(event.FinishTask).Time).To(BeNumerically("<=", time.Now().Unix(), 1))
					Expect(savedEvent.(event.FinishTask).Origin).To(Equal(event.Origin{
						ID: originID,
					}))

				})
			})
		})

		Describe("Failed", func() {
			JustBeforeEach(func() {
				executionDelegate.Failed(errors.New("nope"))
			})

			It("does not save the build's input", func() {
				Expect(fakeBuild.SaveInputCallCount()).To(BeZero())
			})

			It("saves an error event", func() {
				Expect(fakeBuild.SaveEventCallCount()).To(Equal(1))

				savedEvent := fakeBuild.SaveEventArgsForCall(0)
				Expect(savedEvent).To(Equal(event.Error{
					Message: "nope",
					Origin: event.Origin{
						ID: originID,
					},
				}))

			})
		})

		Describe("ImageVersionDetermined", func() {
			var resourceCacheIdentifier worker.ResourceCacheIdentifier

			BeforeEach(func() {
				resourceCacheIdentifier = worker.ResourceCacheIdentifier{
					ResourceVersion: atc.Version{"ref": "asdf"},
					ResourceHash:    "our-super-sweet-resource-hash",
				}
			})

			It("Calls through to the database", func() {
				fakeBuild.SaveImageResourceVersionReturns(nil)

				err := executionDelegate.ImageVersionDetermined(resourceCacheIdentifier)
				Expect(err).ToNot(HaveOccurred())

				Expect(fakeBuild.SaveImageResourceVersionCallCount()).To(Equal(1))
				actualPlanID, actualResourceVersion, actualResourceHash := fakeBuild.SaveImageResourceVersionArgsForCall(0)
				Expect(actualPlanID).To(Equal(atc.PlanID("some-origin-id")))
				Expect(actualResourceVersion).To(Equal(atc.Version{"ref": "asdf"}))
				Expect(actualResourceHash).To(Equal("our-super-sweet-resource-hash"))
			})

			It("Propagates errors", func() {
				distaster := errors.New("sorry mate")
				fakeBuild.SaveImageResourceVersionReturns(distaster)

				err := executionDelegate.ImageVersionDetermined(resourceCacheIdentifier)
				Expect(err).To(Equal(distaster))
			})
		})

		Describe("Stdout", func() {
			var writer io.Writer

			BeforeEach(func() {
				writer = executionDelegate.Stdout()
			})

			It("saves log events with the correct origin", func() {
				_, err := writer.Write([]byte("some stdout"))
				Expect(err).NotTo(HaveOccurred())

				Expect(fakeBuild.SaveEventCallCount()).To(Equal(1))

				savedEvent := fakeBuild.SaveEventArgsForCall(0)
				Expect(savedEvent).To(Equal(event.Log{
					Origin: event.Origin{
						Source: event.OriginSourceStdout,
						ID:     originID,
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
				Expect(err).NotTo(HaveOccurred())

				Expect(fakeBuild.SaveEventCallCount()).To(Equal(1))

				savedEvent := fakeBuild.SaveEventArgsForCall(0)
				Expect(savedEvent).To(Equal(event.Log{
					Origin: event.Origin{
						Source: event.OriginSourceStderr,
						ID:     originID,
					},
					Payload: "some stderr",
				}))

			})
		})
	})

	Describe("OutputDelegate", func() {
		var (
			putPlan atc.PutPlan

			outputDelegate exec.PutDelegate
		)

		BeforeEach(func() {
			putPlan = atc.PutPlan{
				Name:     "some-output-name",
				Resource: "some-output-resource",
				Type:     "some-type",
				Source:   atc.Source{"some": "source"},
				Params:   atc.Params{"some": "params"},
			}

			outputDelegate = delegate.OutputDelegate(logger, putPlan, originID)
		})

		Describe("Initializing", func() {
			JustBeforeEach(func() {
				outputDelegate.Initializing()
			})

			It("saves an initializing event", func() {
				Expect(fakeBuild.SaveEventCallCount()).To(Equal(1))

				savedEvent := fakeBuild.SaveEventArgsForCall(0)
				Expect(savedEvent).To(Equal(event.InitializePut{
					Origin: event.Origin{
						ID: originID,
					},
				}))
			})
		})

		Describe("Completed", func() {
			var versionInfo *exec.VersionInfo

			BeforeEach(func() {
				versionInfo = &exec.VersionInfo{
					Version:  atc.Version{"result": "version"},
					Metadata: []atc.MetadataField{{"result", "metadata"}},
				}
			})

			Context("when the version info is nil", func() {
				JustBeforeEach(func() {
					outputDelegate.Completed(exec.ExitStatus(0), nil)
				})

				It("does not save the build's output", func() {
					Expect(fakeBuild.SaveOutputCallCount()).To(Equal(0))
				})

				It("saves an output event", func() {
					Expect(fakeBuild.SaveEventCallCount()).To(Equal(1))

					savedEvent := fakeBuild.SaveEventArgsForCall(0)
					Expect(savedEvent).To(Equal(event.FinishPut{
						Origin: event.Origin{
							ID: originID,
						},
						Plan: event.PutPlan{
							Name:     "some-output-name",
							Resource: "some-output-resource",
							Type:     "some-type",
						},
						ExitStatus:      0,
						CreatedVersion:  nil,
						CreatedMetadata: nil,
					}))

				})
			})

			Context("when exit status is 0", func() {
				JustBeforeEach(func() {
					outputDelegate.Completed(exec.ExitStatus(0), versionInfo)
				})

				It("saves the build's output", func() {
					Expect(fakeBuild.SaveOutputCallCount()).To(Equal(1))

					savedOutput, explicit := fakeBuild.SaveOutputArgsForCall(0)
					Expect(savedOutput).To(Equal(dbng.VersionedResource{
						Resource: "some-output-resource",
						Type:     "some-type",
						Version:  dbng.ResourceVersion{"result": "version"},
						Metadata: []dbng.ResourceMetadataField{{"result", "metadata"}},
					}))

					Expect(explicit).To(BeTrue())
				})

				It("saves an output event", func() {
					Expect(fakeBuild.SaveEventCallCount()).To(Equal(1))

					savedEvent := fakeBuild.SaveEventArgsForCall(0)
					Expect(savedEvent).To(Equal(event.FinishPut{
						Origin: event.Origin{
							ID: originID,
						},
						Plan: event.PutPlan{
							Name:     "some-output-name",
							Resource: "some-output-resource",
							Type:     "some-type",
						},
						CreatedVersion:  versionInfo.Version,
						CreatedMetadata: versionInfo.Metadata,
						ExitStatus:      0,
					}))

				})
			})

			Context("when exit status is not 0", func() {
				JustBeforeEach(func() {
					outputDelegate.Completed(exec.ExitStatus(72), versionInfo)
				})

				It("saves the build's output", func() {
					Expect(fakeBuild.SaveOutputCallCount()).To(Equal(1))

					savedOutput, explicit := fakeBuild.SaveOutputArgsForCall(0)
					Expect(savedOutput).To(Equal(dbng.VersionedResource{
						Resource: "some-output-resource",
						Type:     "some-type",
						Version:  dbng.ResourceVersion{"result": "version"},
						Metadata: []dbng.ResourceMetadataField{{"result", "metadata"}},
					}))

					Expect(explicit).To(BeTrue())
				})

				It("saves an output event", func() {
					Expect(fakeBuild.SaveEventCallCount()).To(Equal(1))

					savedEvent := fakeBuild.SaveEventArgsForCall(0)
					Expect(savedEvent).To(Equal(event.FinishPut{
						Origin: event.Origin{
							ID: originID,
						},
						Plan: event.PutPlan{
							Name:     "some-output-name",
							Resource: "some-output-resource",
							Type:     "some-type",
						},
						CreatedVersion:  versionInfo.Version,
						CreatedMetadata: versionInfo.Metadata,
						ExitStatus:      72,
					}))

				})

			})

			Describe("Finish", func() {
				var (
					finishErr error
					aborted   bool
					succeeded exec.Success
				)

				Context("without error", func() {
					BeforeEach(func() {
						finishErr = nil
					})

					Context("when it was told it succeeded", func() {
						BeforeEach(func() {
							succeeded = true
							aborted = false
						})

						It("finishes with status 'failed'", func() {
							delegate.Finish(logger, finishErr, succeeded, aborted)

							Expect(fakeBuild.FinishCallCount()).To(Equal(1))

							savedStatus := fakeBuild.FinishArgsForCall(0)
							Expect(savedStatus).To(Equal(dbng.BuildStatusSucceeded))
						})
					})

					Context("when it was told it failed", func() {
						BeforeEach(func() {
							succeeded = false
						})

						It("finishes with status 'failed'", func() {
							delegate.Finish(logger, finishErr, succeeded, aborted)

							Expect(fakeBuild.FinishCallCount()).To(Equal(1))

							savedStatus := fakeBuild.FinishArgsForCall(0)
							Expect(savedStatus).To(Equal(dbng.BuildStatusFailed))
						})
					})
				})
			})
		})

		Describe("Failed", func() {
			JustBeforeEach(func() {
				outputDelegate.Failed(errors.New("nope"))
			})

			It("does not save the build's input", func() {
				Expect(fakeBuild.SaveInputCallCount()).To(BeZero())
			})

			It("saves an error event", func() {
				Expect(fakeBuild.SaveEventCallCount()).To(Equal(1))

				savedEvent := fakeBuild.SaveEventArgsForCall(0)
				Expect(savedEvent).To(Equal(event.Error{
					Origin: event.Origin{
						ID: originID,
					},
					Message: "nope",
				}))

			})
		})

		Describe("ImageVersionDetermined", func() {
			var resourceCacheIdentifier worker.ResourceCacheIdentifier

			BeforeEach(func() {
				resourceCacheIdentifier = worker.ResourceCacheIdentifier{
					ResourceVersion: atc.Version{"ref": "asdf"},
					ResourceHash:    "our-super-sweet-resource-hash",
				}
			})

			It("calls through to the database", func() {
				fakeBuild.SaveImageResourceVersionReturns(nil)

				err := outputDelegate.ImageVersionDetermined(resourceCacheIdentifier)
				Expect(err).ToNot(HaveOccurred())

				Expect(fakeBuild.SaveImageResourceVersionCallCount()).To(Equal(1))
				actualPlanID, actualResourceVersion, actualResourceHash := fakeBuild.SaveImageResourceVersionArgsForCall(0)
				Expect(actualPlanID).To(Equal(atc.PlanID("some-origin-id")))
				Expect(actualResourceVersion).To(Equal(atc.Version{"ref": "asdf"}))
				Expect(actualResourceHash).To(Equal("our-super-sweet-resource-hash"))
			})

			It("propagates errors", func() {
				distaster := errors.New("sorry mate")
				fakeBuild.SaveImageResourceVersionReturns(distaster)

				err := outputDelegate.ImageVersionDetermined(resourceCacheIdentifier)
				Expect(err).To(Equal(distaster))
			})
		})

		Describe("Stdout", func() {
			var writer io.Writer

			BeforeEach(func() {
				writer = outputDelegate.Stdout()
			})

			It("saves log events with the output's origin", func() {
				_, err := writer.Write([]byte("some stdout"))
				Expect(err).NotTo(HaveOccurred())

				Expect(fakeBuild.SaveEventCallCount()).To(Equal(1))

				savedEvent := fakeBuild.SaveEventArgsForCall(0)
				Expect(savedEvent).To(Equal(event.Log{
					Origin: event.Origin{
						Source: event.OriginSourceStdout,
						ID:     originID,
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
				Expect(err).NotTo(HaveOccurred())

				Expect(fakeBuild.SaveEventCallCount()).To(Equal(1))

				savedEvent := fakeBuild.SaveEventArgsForCall(0)
				Expect(savedEvent).To(Equal(event.Log{
					Origin: event.Origin{
						Source: event.OriginSourceStderr,
						ID:     originID,
					},
					Payload: "some stderr",
				}))

			})
		})
	})

	Describe("Aborted", func() {
		var aborted bool

		JustBeforeEach(func() {
			aborted = true
		})

		Describe("Finish", func() {
			var (
				finishErr error
				succeeded exec.Success
				// aborted   bool
			)

			Context("with success", func() {
				BeforeEach(func() {
					finishErr = nil
					succeeded = true

				})

				It("finishes with status 'aborted'", func() {
					delegate.Finish(logger, finishErr, succeeded, aborted)

					Expect(fakeBuild.FinishCallCount()).To(Equal(1))

					savedStatus := fakeBuild.FinishArgsForCall(0)
					Expect(savedStatus).To(Equal(dbng.BuildStatusAborted))
				})
			})

			Context("with failure", func() {
				disaster := errors.New("nope")

				BeforeEach(func() {
					finishErr = disaster
					succeeded = false
				})

				It("finishes with status 'aborted'", func() {
					delegate.Finish(logger, finishErr, succeeded, aborted)

					Expect(fakeBuild.FinishCallCount()).To(Equal(1))

					savedStatus := fakeBuild.FinishArgsForCall(0)
					Expect(savedStatus).To(Equal(dbng.BuildStatusAborted))
				})
			})
		})
	})
})
