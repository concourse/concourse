package builder_test

import (
	"errors"
	"github.com/concourse/concourse/atc/runtime"
	"io"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"code.cloudfoundry.org/clock/fakeclock"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/db/dbfakes"
	"github.com/concourse/concourse/atc/engine/builder"
	"github.com/concourse/concourse/atc/event"
	"github.com/concourse/concourse/atc/exec"
	"github.com/concourse/concourse/vars"
)

var _ = Describe("DelegateFactory", func() {
	var (
		logger          *lagertest.TestLogger
		fakeBuild       *dbfakes.FakeBuild
		fakePipeline    *dbfakes.FakePipeline
		fakeResource    *dbfakes.FakeResource
		fakeClock       *fakeclock.FakeClock
		credVarsTracker vars.CredVarsTracker
	)

	BeforeEach(func() {
		logger = lagertest.NewTestLogger("test")

		fakeBuild = new(dbfakes.FakeBuild)
		fakePipeline = new(dbfakes.FakePipeline)
		fakeResource = new(dbfakes.FakeResource)
		fakeClock = fakeclock.NewFakeClock(time.Unix(123456789, 0))
		credVars := vars.StaticVariables{
			"source-param": "super-secret-source",
			"git-key":      "{\n123\n456\n789\n}\n",
		}
		credVarsTracker = vars.NewCredVarsTracker(credVars, true)
	})

	Describe("GetDelegate", func() {
		var (
			delegate   exec.GetDelegate
			info       runtime.VersionResult
			exitStatus exec.ExitStatus
		)

		BeforeEach(func() {
			info = runtime.VersionResult{
				Version:  atc.Version{"foo": "bar"},
				Metadata: []atc.MetadataField{{Name: "baz", Value: "shmaz"}},
			}

			delegate = builder.NewGetDelegate(fakeBuild, "some-plan-id", credVarsTracker, fakeClock)
		})

		Describe("Finished", func() {
			JustBeforeEach(func() {
				delegate.Finished(logger, exitStatus, info)
			})

			It("saves an event", func() {
				Expect(fakeBuild.SaveEventCallCount()).To(Equal(1))
				Expect(fakeBuild.SaveEventArgsForCall(0)).To(Equal(event.FinishGet{
					Origin:          event.Origin{ID: event.OriginID("some-plan-id")},
					Time:            123456789,
					ExitStatus:      int(exitStatus),
					FetchedVersion:  info.Version,
					FetchedMetadata: info.Metadata,
				}))
			})
		})

		Describe("UpdateVersion", func() {
			JustBeforeEach(func() {
				plan := atc.GetPlan{Resource: "some-resource"}
				delegate.UpdateVersion(logger, plan, info)
			})

			Context("when retrieving the pipeline fails", func() {
				BeforeEach(func() {
					fakeBuild.PipelineReturns(nil, false, errors.New("nope"))
				})

				It("doesn't update the metadata", func() {
					Expect(fakeResource.UpdateMetadataCallCount()).To(Equal(0))
				})
			})

			Context("when retrieving the pipeline succeeds", func() {

				Context("when the pipeline is not found", func() {
					BeforeEach(func() {
						fakeBuild.PipelineReturns(nil, false, nil)
					})

					It("doesn't update the metadata", func() {
						Expect(fakeResource.UpdateMetadataCallCount()).To(Equal(0))
					})
				})

				Context("when the pipeline is found", func() {
					BeforeEach(func() {
						fakeBuild.PipelineReturns(fakePipeline, true, nil)
					})

					Context("when retrieving the resource fails", func() {
						BeforeEach(func() {
							fakePipeline.ResourceReturns(nil, false, errors.New("nope"))
						})

						It("doesn't update the metadata", func() {
							Expect(fakeResource.UpdateMetadataCallCount()).To(Equal(0))
						})
					})

					Context("when retrieving the resource succeeds", func() {

						It("retrives the resource by name", func() {
							Expect(fakePipeline.ResourceArgsForCall(0)).To(Equal("some-resource"))
						})

						Context("when the resource is not found", func() {
							BeforeEach(func() {
								fakePipeline.ResourceReturns(nil, false, nil)
							})

							It("doesn't update the metadata", func() {
								Expect(fakeResource.UpdateMetadataCallCount()).To(Equal(0))
							})
						})

						Context("when the resource is found", func() {
							BeforeEach(func() {
								fakePipeline.ResourceReturns(fakeResource, true, nil)
							})

							It("updates the metadata", func() {
								Expect(fakeResource.UpdateMetadataCallCount()).To(Equal(1))
								version, metadata := fakeResource.UpdateMetadataArgsForCall(0)
								Expect(version).To(Equal(info.Version))
								Expect(metadata).To(Equal(db.NewResourceConfigMetadataFields(info.Metadata)))
							})
						})
					})
				})
			})
		})
	})

	Describe("PutDelegate", func() {
		var (
			delegate   exec.PutDelegate
			info       runtime.VersionResult
			exitStatus exec.ExitStatus
		)

		BeforeEach(func() {
			info = runtime.VersionResult{
				Version:  atc.Version{"foo": "bar"},
				Metadata: []atc.MetadataField{{Name: "baz", Value: "shmaz"}},
			}

			delegate = builder.NewPutDelegate(fakeBuild, "some-plan-id", credVarsTracker, fakeClock)
		})

		Describe("Finished", func() {
			JustBeforeEach(func() {
				delegate.Finished(logger, exitStatus, info)
			})

			It("saves an event", func() {
				Expect(fakeBuild.SaveEventCallCount()).To(Equal(1))
				Expect(fakeBuild.SaveEventArgsForCall(0)).To(Equal(event.FinishPut{
					Origin:          event.Origin{ID: event.OriginID("some-plan-id")},
					Time:            123456789,
					ExitStatus:      int(exitStatus),
					CreatedVersion:  info.Version,
					CreatedMetadata: info.Metadata,
				}))
			})
		})

		Describe("SaveOutput", func() {
			var plan atc.PutPlan
			var source atc.Source
			var resourceTypes atc.VersionedResourceTypes

			JustBeforeEach(func() {
				plan = atc.PutPlan{
					Name:     "some-name",
					Type:     "some-type",
					Resource: "some-resource",
				}
				source = atc.Source{"some": "source"}
				resourceTypes = atc.VersionedResourceTypes{}

				delegate.SaveOutput(logger, plan, source, resourceTypes, info)
			})

			It("saves the build output", func() {
				Expect(fakeBuild.SaveOutputCallCount()).To(Equal(1))
				resourceType, sourceArg, resourceTypesArg, version, metadata, name, resource := fakeBuild.SaveOutputArgsForCall(0)
				Expect(resourceType).To(Equal(plan.Type))
				Expect(sourceArg).To(Equal(source))
				Expect(resourceTypesArg).To(Equal(resourceTypes))
				Expect(version).To(Equal(info.Version))
				Expect(metadata).To(Equal(db.NewResourceConfigMetadataFields(info.Metadata)))
				Expect(name).To(Equal(plan.Name))
				Expect(resource).To(Equal(plan.Resource))
			})
		})
	})

	Describe("TaskDelegate", func() {
		var (
			delegate   exec.TaskDelegate
			config     atc.TaskConfig
			exitStatus exec.ExitStatus
		)

		BeforeEach(func() {
			delegate = builder.NewTaskDelegate(fakeBuild, "some-plan-id", credVarsTracker, fakeClock)
		})

		Describe("Initializing", func() {
			JustBeforeEach(func() {
				delegate.Initializing(logger, config)
			})

			It("saves an event", func() {
				Expect(fakeBuild.SaveEventCallCount()).To(Equal(1))
				event := fakeBuild.SaveEventArgsForCall(0)
				Expect(event.EventType()).To(Equal(atc.EventType("initialize-task")))
			})
		})

		Describe("Starting", func() {
			JustBeforeEach(func() {
				delegate.Starting(logger, config)
			})

			It("saves an event", func() {
				Expect(fakeBuild.SaveEventCallCount()).To(Equal(1))
				event := fakeBuild.SaveEventArgsForCall(0)
				Expect(event.EventType()).To(Equal(atc.EventType("start-task")))
			})
		})

		Describe("Finished", func() {
			JustBeforeEach(func() {
				delegate.Finished(logger, exitStatus)
			})

			It("saves an event", func() {
				Expect(fakeBuild.SaveEventCallCount()).To(Equal(1))
				event := fakeBuild.SaveEventArgsForCall(0)
				Expect(event.EventType()).To(Equal(atc.EventType("finish-task")))
			})
		})
	})

	Describe("CheckDelegate", func() {
		var (
			delegate  exec.CheckDelegate
			fakeCheck *dbfakes.FakeCheck
			versions  []atc.Version
		)

		BeforeEach(func() {
			fakeCheck = new(dbfakes.FakeCheck)

			delegate = builder.NewCheckDelegate(fakeCheck, "some-plan-id", credVarsTracker, fakeClock)
			versions = []atc.Version{{"some": "version"}}
		})

		Describe("SaveVersions", func() {
			JustBeforeEach(func() {
				Expect(delegate.SaveVersions(versions)).To(Succeed())
			})

			It("saves an event", func() {
				Expect(fakeCheck.SaveVersionsCallCount()).To(Equal(1))
				actualVersions := fakeCheck.SaveVersionsArgsForCall(0)
				Expect(actualVersions).To(Equal(versions))
			})
		})
	})

	Describe("BuildStepDelegate", func() {
		var (
			delegate exec.BuildStepDelegate
		)

		BeforeEach(func() {
			delegate = builder.NewBuildStepDelegate(fakeBuild, "some-plan-id", credVarsTracker, fakeClock)
		})

		Describe("Initializing", func() {
			JustBeforeEach(func() {
				delegate.Initializing(logger)
			})

			It("saves an event", func() {
				Expect(fakeBuild.SaveEventCallCount()).To(Equal(1))
				event := fakeBuild.SaveEventArgsForCall(0)
				Expect(event.EventType()).To(Equal(atc.EventType("initialize")))
			})
		})

		Describe("Finished", func() {
			JustBeforeEach(func() {
				delegate.Finished(logger, true)
			})

			It("saves an event", func() {
				Expect(fakeBuild.SaveEventCallCount()).To(Equal(1))
				event := fakeBuild.SaveEventArgsForCall(0)
				Expect(event.EventType()).To(Equal(atc.EventType("finish")))
			})
		})

		Describe("ImageVersionDetermined", func() {
			var fakeResourceCache *dbfakes.FakeUsedResourceCache

			BeforeEach(func() {
				fakeResourceCache = new(dbfakes.FakeUsedResourceCache)
				fakeResourceCache.IDReturns(42)
			})

			JustBeforeEach(func() {
				Expect(delegate.ImageVersionDetermined(fakeResourceCache)).To(Succeed())
			})

			It("records the resource cache as an image resource for the build", func() {
				Expect(fakeBuild.SaveImageResourceVersionCallCount()).To(Equal(1))
				Expect(fakeBuild.SaveImageResourceVersionArgsForCall(0)).To(Equal(fakeResourceCache))
			})
		})

		Describe("Stdout", func() {
			var writer io.Writer

			BeforeEach(func() {
				writer = delegate.Stdout()
			})

			Describe("writing to the writer", func() {
				var writtenBytes int
				var writeErr error

				JustBeforeEach(func() {
					writtenBytes, writeErr = writer.Write([]byte("hello\nworld"))
					writer.(io.Closer).Close()
				})

				Context("when saving the event succeeds", func() {
					BeforeEach(func() {
						fakeBuild.SaveEventReturns(nil)
					})

					It("returns the length of the string, and no error", func() {
						Expect(writtenBytes).To(Equal(len("hello\nworld")))
						Expect(writeErr).ToNot(HaveOccurred())
					})

					It("saves a log event", func() {
						Expect(fakeBuild.SaveEventCallCount()).To(Equal(2))
						Expect(fakeBuild.SaveEventArgsForCall(0)).To(Equal(event.Log{
							Time:    123456789,
							Payload: "hello\n",
							Origin: event.Origin{
								Source: event.OriginSourceStdout,
								ID:     "some-plan-id",
							},
						}))
						Expect(fakeBuild.SaveEventArgsForCall(1)).To(Equal(event.Log{
							Time:    123456789,
							Payload: "world",
							Origin: event.Origin{
								Source: event.OriginSourceStdout,
								ID:     "some-plan-id",
							},
						}))
					})
				})

				Context("when saving the event fails", func() {
					disaster := errors.New("nope")

					BeforeEach(func() {
						fakeBuild.SaveEventReturns(disaster)
					})

					It("returns 0 length, and the error", func() {
						Expect(writtenBytes).To(Equal(0))
						Expect(writeErr).To(Equal(disaster))
					})
				})
			})
		})

		Describe("Stderr", func() {
			var writer io.Writer

			BeforeEach(func() {
				writer = delegate.Stderr()
			})

			Describe("writing to the writer", func() {
				var writtenBytes int
				var writeErr error

				JustBeforeEach(func() {
					writtenBytes, writeErr = writer.Write([]byte("hello\n"))
					writer.(io.Closer).Close()
				})

				Context("when saving the event succeeds", func() {
					BeforeEach(func() {
						fakeBuild.SaveEventReturns(nil)
					})

					It("returns the length of the string, and no error", func() {
						Expect(writtenBytes).To(Equal(len("hello\n")))
						Expect(writeErr).ToNot(HaveOccurred())
					})

					It("saves a log event", func() {
						Expect(fakeBuild.SaveEventCallCount()).To(Equal(1))
						Expect(fakeBuild.SaveEventArgsForCall(0)).To(Equal(event.Log{
							Time:    123456789,
							Payload: "hello\n",
							Origin: event.Origin{
								Source: event.OriginSourceStderr,
								ID:     "some-plan-id",
							},
						}))
					})
				})

				Context("when saving the event fails", func() {
					disaster := errors.New("nope")

					BeforeEach(func() {
						fakeBuild.SaveEventReturns(disaster)
					})

					It("returns 0 length, and the error", func() {
						Expect(writtenBytes).To(Equal(0))
						Expect(writeErr).To(Equal(disaster))
					})
				})
			})
		})

		Describe("Errored", func() {
			JustBeforeEach(func() {
				delegate.Errored(logger, "fake error message")
			})

			Context("when saving the event succeeds", func() {
				BeforeEach(func() {
					fakeBuild.SaveEventReturns(nil)
				})

				It("saves it with the current time", func() {
					Expect(fakeBuild.SaveEventCallCount()).To(Equal(1))
					Expect(fakeBuild.SaveEventArgsForCall(0)).To(Equal(event.Error{
						Time:    123456789,
						Message: "fake error message",
						Origin: event.Origin{
							ID: "some-plan-id",
						},
					}))
				})
			})

			Context("when saving the event fails", func() {
				disaster := errors.New("nope")

				BeforeEach(func() {
					fakeBuild.SaveEventReturns(disaster)
				})

				It("logs an error", func() {
					logs := logger.Logs()
					Expect(len(logs)).To(Equal(1))
					Expect(logs[0].Message).To(Equal("test.failed-to-save-error-event"))
					Expect(logs[0].Data).To(Equal(lager.Data{"error": "nope"}))
				})
			})
		})

		Describe("No line buffer without secrets redaction", func() {
			BeforeEach(func() {
				credVars := vars.StaticVariables{}
				credVarsTracker = vars.NewCredVarsTracker(credVars, false)
				delegate = builder.NewBuildStepDelegate(fakeBuild, "some-plan-id", credVarsTracker, fakeClock)
			})

			Context("Stdout", func() {
				It("should not buffer lines", func() {
					writer := delegate.Stdout()
					writtenBytes, writeErr := writer.Write([]byte("1\r"))
					Expect(writeErr).To(BeNil())
					Expect(writtenBytes).To(Equal(len("1\r")))
					writtenBytes, writeErr = writer.Write([]byte("2\r"))
					Expect(writeErr).To(BeNil())
					Expect(writtenBytes).To(Equal(len("2\r")))
					writtenBytes, writeErr = writer.Write([]byte("3\r"))
					Expect(writeErr).To(BeNil())
					Expect(writtenBytes).To(Equal(len("3\r")))
					writeErr = writer.(io.Closer).Close()
					Expect(writeErr).To(BeNil())

					Expect(fakeBuild.SaveEventCallCount()).To(Equal(3))
					Expect(fakeBuild.SaveEventArgsForCall(0)).To(Equal(event.Log{
						Time:    123456789,
						Payload: "1\r",
						Origin: event.Origin{
							Source: event.OriginSourceStdout,
							ID:     "some-plan-id",
						},
					}))
					Expect(fakeBuild.SaveEventArgsForCall(1)).To(Equal(event.Log{
						Time:    123456789,
						Payload: "2\r",
						Origin: event.Origin{
							Source: event.OriginSourceStdout,
							ID:     "some-plan-id",
						},
					}))
					Expect(fakeBuild.SaveEventArgsForCall(2)).To(Equal(event.Log{
						Time:    123456789,
						Payload: "3\r",
						Origin: event.Origin{
							Source: event.OriginSourceStdout,
							ID:     "some-plan-id",
						},
					}))
				})
			})

			Context("Stderr", func() {
				It("should not buffer lines", func() {
					writer := delegate.Stderr()
					writtenBytes, writeErr := writer.Write([]byte("1\r"))
					Expect(writeErr).To(BeNil())
					Expect(writtenBytes).To(Equal(len("1\r")))
					writtenBytes, writeErr = writer.Write([]byte("2\r"))
					Expect(writeErr).To(BeNil())
					Expect(writtenBytes).To(Equal(len("2\r")))
					writtenBytes, writeErr = writer.Write([]byte("3\r"))
					Expect(writeErr).To(BeNil())
					Expect(writtenBytes).To(Equal(len("3\r")))
					writeErr = writer.(io.Closer).Close()
					Expect(writeErr).To(BeNil())

					Expect(fakeBuild.SaveEventCallCount()).To(Equal(3))
					Expect(fakeBuild.SaveEventArgsForCall(0)).To(Equal(event.Log{
						Time:    123456789,
						Payload: "1\r",
						Origin: event.Origin{
							Source: event.OriginSourceStderr,
							ID:     "some-plan-id",
						},
					}))
					Expect(fakeBuild.SaveEventArgsForCall(1)).To(Equal(event.Log{
						Time:    123456789,
						Payload: "2\r",
						Origin: event.Origin{
							Source: event.OriginSourceStderr,
							ID:     "some-plan-id",
						},
					}))
					Expect(fakeBuild.SaveEventArgsForCall(2)).To(Equal(event.Log{
						Time:    123456789,
						Payload: "3\r",
						Origin: event.Origin{
							Source: event.OriginSourceStderr,
							ID:     "some-plan-id",
						},
					}))
				})
			})
		})

		Describe("Secrets redaction", func() {
			var (
				writer       io.Writer
				writtenBytes int
				writeErr     error
			)

			BeforeEach(func() {
				delegate.Variables().Get(vars.VariableDefinition{Name: "source-param"})
				delegate.Variables().Get(vars.VariableDefinition{Name: "git-key"})
			})

			Context("Stdout", func() {
				Context("single-line secret", func() {
					JustBeforeEach(func() {
						writer = delegate.Stdout()
						writtenBytes, writeErr = writer.Write([]byte("ok super-secret-source ok"))
						writer.(io.Closer).Close()
					})

					It("should be redacted", func() {
						Expect(writeErr).To(BeNil())
						Expect(writtenBytes).To(Equal(len("ok super-secret-source ok")))
						Expect(fakeBuild.SaveEventCallCount()).To(Equal(1))
						Expect(fakeBuild.SaveEventArgsForCall(0)).To(Equal(event.Log{
							Time:    123456789,
							Payload: "ok ((redacted)) ok",
							Origin: event.Origin{
								Source: event.OriginSourceStdout,
								ID:     "some-plan-id",
							},
						}))
					})
				})

				Context("multi-line secret", func() {
					var logLines string

					JustBeforeEach(func() {
						logLines = "ok123ok\nok456ok\nok789ok\n"
						writer = delegate.Stdout()
						writtenBytes, writeErr = writer.Write([]byte(logLines))
						writer.(io.Closer).Close()
					})

					It("should be redacted", func() {
						Expect(writeErr).To(BeNil())
						Expect(writtenBytes).To(Equal(len(logLines)))
						Expect(fakeBuild.SaveEventCallCount()).To(Equal(1))
						Expect(fakeBuild.SaveEventArgsForCall(0)).To(Equal(event.Log{
							Time:    123456789,
							Payload: "ok((redacted))ok\nok((redacted))ok\nok((redacted))ok\n",
							Origin: event.Origin{
								Source: event.OriginSourceStdout,
								ID:     "some-plan-id",
							},
						}))
					})
				})

				Context("multi-line secret with random log chunk", func() {
					JustBeforeEach(func() {
						writer = delegate.Stdout()
						writtenBytes, writeErr = writer.Write([]byte("ok123ok\nok4"))
						writtenBytes, writeErr = writer.Write([]byte("56ok\nok789ok\n"))
						writer.(io.Closer).Close()
					})

					It("should be redacted", func() {
						Expect(fakeBuild.SaveEventCallCount()).To(Equal(2))
						Expect(fakeBuild.SaveEventArgsForCall(0)).To(Equal(event.Log{
							Time:    123456789,
							Payload: "ok((redacted))ok\n",
							Origin: event.Origin{
								Source: event.OriginSourceStdout,
								ID:     "some-plan-id",
							},
						}))
						Expect(fakeBuild.SaveEventArgsForCall(1)).To(Equal(event.Log{
							Time:    123456789,
							Payload: "ok((redacted))ok\nok((redacted))ok\n",
							Origin: event.Origin{
								Source: event.OriginSourceStdout,
								ID:     "some-plan-id",
							},
						}))
					})
				})
			})

			Context("Stderr", func() {
				Context("single-line secret", func() {
					JustBeforeEach(func() {
						writer = delegate.Stderr()
						writtenBytes, writeErr = writer.Write([]byte("ok super-secret-source ok"))
						writer.(io.Closer).Close()
					})

					It("should be redacted", func() {
						Expect(writeErr).To(BeNil())
						Expect(writtenBytes).To(Equal(len("ok super-secret-source ok")))
						Expect(fakeBuild.SaveEventCallCount()).To(Equal(1))
						Expect(fakeBuild.SaveEventArgsForCall(0)).To(Equal(event.Log{
							Time:    123456789,
							Payload: "ok ((redacted)) ok",
							Origin: event.Origin{
								Source: event.OriginSourceStderr,
								ID:     "some-plan-id",
							},
						}))
					})
				})

				Context("multi-line secret", func() {
					var logLines string

					JustBeforeEach(func() {
						logLines = "{\nok123ok\nok456ok\nok789ok\n}\n"
						writer = delegate.Stderr()
						writtenBytes, writeErr = writer.Write([]byte(logLines))
						writer.(io.Closer).Close()
					})

					It("should be redacted", func() {
						Expect(writeErr).To(BeNil())
						Expect(writtenBytes).To(Equal(len(logLines)))
						Expect(fakeBuild.SaveEventCallCount()).To(Equal(1))
						Expect(fakeBuild.SaveEventArgsForCall(0)).To(Equal(event.Log{
							Time:    123456789,
							Payload: "{\nok((redacted))ok\nok((redacted))ok\nok((redacted))ok\n}\n",
							Origin: event.Origin{
								Source: event.OriginSourceStderr,
								ID:     "some-plan-id",
							},
						}))
					})
				})

				Context("multi-line secret with random log chunk", func() {
					JustBeforeEach(func() {
						writer = delegate.Stderr()
						writtenBytes, writeErr = writer.Write([]byte("ok123ok\nok4"))
						writtenBytes, writeErr = writer.Write([]byte("56ok\nok789ok\n"))
						writer.(io.Closer).Close()
					})

					It("should be redacted", func() {
						Expect(fakeBuild.SaveEventCallCount()).To(Equal(2))
						Expect(fakeBuild.SaveEventArgsForCall(0)).To(Equal(event.Log{
							Time:    123456789,
							Payload: "ok((redacted))ok\n",
							Origin: event.Origin{
								Source: event.OriginSourceStderr,
								ID:     "some-plan-id",
							},
						}))
						Expect(fakeBuild.SaveEventArgsForCall(1)).To(Equal(event.Log{
							Time:    123456789,
							Payload: "ok((redacted))ok\nok((redacted))ok\n",
							Origin: event.Origin{
								Source: event.OriginSourceStderr,
								ID:     "some-plan-id",
							},
						}))
					})
				})
			})
		})
	})
})
