package builder_test

import (
	"errors"
	"io"
	"time"

	"code.cloudfoundry.org/clock/fakeclock"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db/dbfakes"
	"github.com/concourse/concourse/atc/engine/builder"
	"github.com/concourse/concourse/atc/event"
	"github.com/concourse/concourse/atc/exec"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("DelegateFactory", func() {
	var (
		logger    lager.Logger
		fakeBuild *dbfakes.FakeBuild
		fakeClock *fakeclock.FakeClock
	)

	BeforeEach(func() {
		logger = lagertest.NewTestLogger("test")

		fakeBuild = new(dbfakes.FakeBuild)
		fakeClock = fakeclock.NewFakeClock(time.Unix(123456789, 0))
	})

	Describe("GetDelegate", func() {
		var (
			delegate   exec.GetDelegate
			info       exec.VersionInfo
			exitStatus exec.ExitStatus
		)

		BeforeEach(func() {
			info = exec.VersionInfo{
				Version:  atc.Version{"foo": "bar"},
				Metadata: []atc.MetadataField{{Name: "baz", Value: "shmaz"}},
			}

			delegate = builder.NewGetDelegate(fakeBuild, "some-plan-id", fakeClock)
		})

		Describe("Finished", func() {
			JustBeforeEach(func() {
				delegate.Finished(logger, exitStatus, info)
			})

			It("saves an event", func() {
				Expect(fakeBuild.SaveEventCallCount()).To(Equal(1))
				Expect(fakeBuild.SaveEventArgsForCall(0)).To(Equal(event.FinishGet{
					Origin:          event.Origin{ID: event.OriginID("some-plan-id")},
					ExitStatus:      int(exitStatus),
					FetchedVersion:  info.Version,
					FetchedMetadata: info.Metadata,
				}))
			})
		})
	})

	Describe("PutDelegate", func() {
		var (
			delegate   exec.PutDelegate
			info       exec.VersionInfo
			exitStatus exec.ExitStatus
		)

		BeforeEach(func() {
			info = exec.VersionInfo{
				Version:  atc.Version{"foo": "bar"},
				Metadata: []atc.MetadataField{{Name: "baz", Value: "shmaz"}},
			}

			delegate = builder.NewPutDelegate(fakeBuild, "some-plan-id", fakeClock)
		})

		Describe("Finished", func() {
			JustBeforeEach(func() {
				delegate.Finished(logger, exitStatus, info)
			})

			It("saves an event", func() {
				Expect(fakeBuild.SaveEventCallCount()).To(Equal(1))
				Expect(fakeBuild.SaveEventArgsForCall(0)).To(Equal(event.FinishPut{
					Origin:          event.Origin{ID: event.OriginID("some-plan-id")},
					ExitStatus:      int(exitStatus),
					CreatedVersion:  info.Version,
					CreatedMetadata: info.Metadata,
				}))
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
			delegate = builder.NewTaskDelegate(fakeBuild, "some-plan-id", fakeClock)
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

	Describe("BuildStepDelegate", func() {
		var (
			delegate exec.BuildStepDelegate
		)

		BeforeEach(func() {
			delegate = builder.NewBuildStepDelegate(fakeBuild, "some-plan-id", fakeClock)
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
					writtenBytes, writeErr = writer.Write([]byte("hello"))
				})

				Context("when saving the event succeeds", func() {
					BeforeEach(func() {
						fakeBuild.SaveEventReturns(nil)
					})

					It("returns the length of the string, and no error", func() {
						Expect(writtenBytes).To(Equal(len("hello")))
						Expect(writeErr).ToNot(HaveOccurred())
					})

					It("saves a log event", func() {
						Expect(fakeBuild.SaveEventCallCount()).To(Equal(1))
						Expect(fakeBuild.SaveEventArgsForCall(0)).To(Equal(event.Log{
							Time:    123456789,
							Payload: "hello",
							Origin: event.Origin{
								Source: event.OriginSourceStdout,
								ID:     "some-plan-id",
							},
						}))
					})
				})

				Context("when saving the event succeeds", func() {
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
					writtenBytes, writeErr = writer.Write([]byte("hello"))
				})

				Context("when saving the event succeeds", func() {
					BeforeEach(func() {
						fakeBuild.SaveEventReturns(nil)
					})

					It("returns the length of the string, and no error", func() {
						Expect(writtenBytes).To(Equal(len("hello")))
						Expect(writeErr).ToNot(HaveOccurred())
					})

					It("saves a log event", func() {
						Expect(fakeBuild.SaveEventCallCount()).To(Equal(1))
						Expect(fakeBuild.SaveEventArgsForCall(0)).To(Equal(event.Log{
							Time:    123456789,
							Payload: "hello",
							Origin: event.Origin{
								Source: event.OriginSourceStderr,
								ID:     "some-plan-id",
							},
						}))
					})
				})

				Context("when saving the event succeeds", func() {
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
	})
})
