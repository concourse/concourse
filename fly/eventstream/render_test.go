package eventstream_test

import (
	"io"
	"time"

	"github.com/fatih/color"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/onsi/gomega/gbytes"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/event"
	"github.com/concourse/concourse/fly/eventstream"
	"github.com/concourse/concourse/fly/ui"
	"github.com/concourse/concourse/go-concourse/concourse/eventstream/eventstreamfakes"
)

var _ = Describe("V1.0 Renderer", func() {
	var (
		out     *gbytes.Buffer
		stream  *eventstreamfakes.FakeEventStream
		options eventstream.RenderOptions

		receivedEvents chan<- atc.Event

		exitStatus int
	)

	BeforeEach(func() {
		color.NoColor = false
		out = gbytes.NewBuffer()
		stream = new(eventstreamfakes.FakeEventStream)
		options = eventstream.RenderOptions{}

		events := make(chan atc.Event, 100)
		receivedEvents = events

		stream.NextEventStub = func() (atc.Event, error) {
			select {
			case ev := <-events:
				return ev, nil
			default:
				return nil, io.EOF
			}
		}
	})

	JustBeforeEach(func() {
		exitStatus = eventstream.Render(out, stream, options)
	})

	Context("when a Log event is received", func() {
		BeforeEach(func() {
			receivedEvents <- event.Log{
				Payload: "hello",
				Time:    time.Now().Unix(),
			}
		})

		It("prints its payload", func() {
			Expect(out).To(gbytes.Say("hello"))
		})

		Context("and time configuration is enabled", func() {
			BeforeEach(func() {
				options.ShowTimestamp = true
			})

			It("prints its payload with a timestamp", func() {
				Expect(out).To(gbytes.Say(`\d{2}\:\d{2}\:\d{2}\s{2}hello`))
			})

		})
	})

	Context("when an Error event is received", func() {
		BeforeEach(func() {
			receivedEvents <- event.Error{
				Message: "oh no!",
			}
		})

		It("prints its message in bold red, followed by a linebreak", func() {
			Expect(out.Contents()).To(ContainSubstring(ui.ErroredColor.SprintFunc()("oh no!") + "\n"))
		})

		Context("and time configuration is enabled", func() {
			BeforeEach(func() {
				options.ShowTimestamp = true
			})

			It("empty space is prefixed", func() {
				Expect(out).To(gbytes.Say(`\s{10}\w*`))
			})
		})
	})

	Context("when an InitializeTask event is received", func() {
		BeforeEach(func() {
			receivedEvents <- event.InitializeTask{
				Time: time.Now().Unix(),
			}
		})

		It("prints initializing", func() {
			Expect(out.Contents()).To(ContainSubstring("\x1b[1minitializing\x1b[0m\n"))
		})

		Context("and time configuration is enabled", func() {
			BeforeEach(func() {
				options.ShowTimestamp = true
			})

			It("timestamp is prefixed", func() {
				Expect(out).To(gbytes.Say(`\d{2}\:\d{2}\:\d{2}\s{2}\w*`))
			})
		})
	})

	Context("and a StartTask event is received", func() {
		BeforeEach(func() {
			receivedEvents <- event.StartTask{
				Time: time.Now().Unix(),
				TaskConfig: event.TaskConfig{
					Image: "some-image",
					Run: event.TaskRunConfig{
						Path: "/some/script",
						Args: []string{"arg1", "arg2"},
					},
				},
			}
		})

		It("prints the build's run script", func() {
			Expect(out.Contents()).To(ContainSubstring("\x1b[1mrunning /some/script arg1 arg2\x1b[0m\n"))
		})

		Context("and time configuration enabled", func() {
			BeforeEach(func() {
				options.ShowTimestamp = true
			})

			It("timestamp is prefixed", func() {
				Expect(out).To(gbytes.Say(`\d{2}\:\d{2}\:\d{2}\s{2}\w*`))
			})
		})
	})

	Context("when a FinishTask event is received", func() {
		BeforeEach(func() {
			receivedEvents <- event.FinishTask{
				ExitStatus: 42,
			}
		})

		It("returns its exit status", func() {
			Expect(exitStatus).To(Equal(42))
		})

		Context("and a Status event is received", func() {
			BeforeEach(func() {
				receivedEvents <- event.Status{
					Status: atc.StatusSucceeded,
				}
			})

			It("still processes it", func() {
				Expect(out.Contents()).To(ContainSubstring("succeeded"))
			})

			It("exits with the status from the FinishTask event", func() {
				Expect(exitStatus).To(Equal(42))
			})

			Context("and time configuration is enabled", func() {
				BeforeEach(func() {
					options.ShowTimestamp = true
				})

				It("empty string is prefixed", func() {
					Expect(out).To(gbytes.Say(`\s{10}\w*`))
				})
			})
		})
	})

	Describe("receiving a Status event", func() {
		Context("with status 'succeeded'", func() {
			BeforeEach(func() {
				receivedEvents <- event.Status{
					Status: atc.StatusSucceeded,
					Time:   time.Now().Unix(),
				}
			})

			It("prints it in green", func() {
				Expect(out.Contents()).To(ContainSubstring(ui.SucceededColor.SprintFunc()("succeeded") + "\n"))
			})

			It("exits 0", func() {
				Expect(exitStatus).To(Equal(0))
			})

			Context("and time configuration is enabled", func() {
				BeforeEach(func() {
					options.ShowTimestamp = true
				})

				It("timestamp is prefixed", func() {
					Expect(out).To(gbytes.Say(`\d{2}\:\d{2}\:\d{2}\s{2}\w*`))
				})
			})
		})

		Context("with status 'failed'", func() {
			BeforeEach(func() {
				receivedEvents <- event.Status{
					Status: atc.StatusFailed,
					Time:   time.Now().Unix(),
				}
			})

			It("prints it in red", func() {
				Expect(out.Contents()).To(ContainSubstring(ui.FailedColor.SprintFunc()("failed") + "\n"))
			})

			It("exits 1", func() {
				Expect(exitStatus).To(Equal(1))
			})

			Context("and time configuration is enabled", func() {
				BeforeEach(func() {
					options.ShowTimestamp = true
				})

				It("timestamp is prefixed", func() {
					Expect(out).To(gbytes.Say(`\d{2}\:\d{2}\:\d{2}\s{2}\w*`))
				})
			})
		})

		Context("with status 'errored'", func() {
			BeforeEach(func() {
				receivedEvents <- event.Status{
					Status: atc.StatusErrored,
					Time:   time.Now().Unix(),
				}
			})

			It("prints it in bold red", func() {
				Expect(out.Contents()).To(ContainSubstring(ui.ErroredColor.SprintFunc()("errored") + "\n"))
			})

			It("exits 2", func() {
				Expect(exitStatus).To(Equal(2))
			})

			Context("and time configuration is enabled", func() {
				BeforeEach(func() {
					options.ShowTimestamp = true
				})

				It("timestamp is prefixed", func() {
					Expect(out).To(gbytes.Say(`\d{2}\:\d{2}\:\d{2}\s{2}\w*`))
				})
			})
		})

		Context("with status 'aborted'", func() {
			BeforeEach(func() {
				receivedEvents <- event.Status{
					Status: atc.StatusAborted,
					Time:   time.Now().Unix(),
				}
			})

			It("prints it in yellow", func() {
				Expect(out.Contents()).To(ContainSubstring(ui.AbortedColor.SprintFunc()("aborted") + "\n"))
			})

			It("exits 3", func() {
				Expect(exitStatus).To(Equal(3))
			})

			Context("and time configuration is enabled", func() {
				BeforeEach(func() {
					options.ShowTimestamp = true
				})

				It("timestamp is prefixed", func() {
					Expect(out).To(gbytes.Say(`\d{2}\:\d{2}\:\d{2}\s{2}\w*`))
				})
			})
		})
	})

	Context("when a WaitingForWorker event is received", func() {
		BeforeEach(func() {
			receivedEvents <- event.WaitingForWorker{
				Time: time.Now().Unix(),
			}
		})

		It("prints the build's run script", func() {
			Expect(out.Contents()).To(ContainSubstring("\x1b[1mno suitable workers found, waiting for worker...\x1b[0m\n"))
		})

		Context("and time configuration enabled", func() {
			BeforeEach(func() {
				options.ShowTimestamp = true
			})

			It("timestamp is prefixed", func() {
				Expect(out).To(gbytes.Say(`\d{2}\:\d{2}\:\d{2}\s{2}\w*`))
			})
		})
	})

	Context("when a SelectedWorker event is received", func() {
		BeforeEach(func() {
			receivedEvents <- event.SelectedWorker{
				Time:       time.Now().Unix(),
				WorkerName: "some-worker",
			}
		})

		It("prints the build's run script", func() {
			Expect(out.Contents()).To(ContainSubstring("\x1b[1mselected worker:\u001B[0m some-worker\n"))
		})

		Context("and time configuration enabled", func() {
			BeforeEach(func() {
				options.ShowTimestamp = true
			})

			It("timestamp is prefixed", func() {
				Expect(out).To(gbytes.Say(`\d{2}\:\d{2}\:\d{2}\s{2}\w*`))
			})
		})
	})

	Context("when a StreamingVolume event is received", func() {
		BeforeEach(func() {
			receivedEvents <- event.StreamingVolume{
				Time:         time.Now().Unix(),
				Volume:       "some-volume",
				SourceWorker: "source-worker",
				DestWorker:   "dest-worker",
			}
		})

		It("prints the event", func() {
			Expect(out.Contents()).To(ContainSubstring("\x1b[1mstreaming volume\u001B[0m some-volume \x1b[1mfrom worker\u001B[0m source-worker\n"))
		})

		Context("and time configuration enabled", func() {
			BeforeEach(func() {
				options.ShowTimestamp = true
			})

			It("timestamp is prefixed", func() {
				Expect(out).To(gbytes.Say(`\d{2}\:\d{2}\:\d{2}\s{2}\w*`))
			})
		})
	})

	Context("when a WaitingForStreamedVolume event is received", func() {
		BeforeEach(func() {
			receivedEvents <- event.WaitingForStreamedVolume{
				Time:       time.Now().Unix(),
				Volume:     "some-volume",
				DestWorker: "dest-worker",
			}
		})

		It("prints the event", func() {
			Expect(out.Contents()).To(ContainSubstring("\x1b[1mwaiting for volume\u001B[0m some-volume \x1b[1mto be streamed by another step\n"))
		})

		Context("and time configuration enabled", func() {
			BeforeEach(func() {
				options.ShowTimestamp = true
			})

			It("timestamp is prefixed", func() {
				Expect(out).To(gbytes.Say(`\d{2}\:\d{2}\:\d{2}\s{2}\w*`))
			})
		})
	})

	Context("when an UnknownEventTypeError or UnknownEventVersionError is received", func() {

		BeforeEach(func() {
			errors := make(chan error, 100)

			stream.NextEventStub = func() (atc.Event, error) {
				select {
				case ev := <-errors:
					return nil, ev
				default:
					return nil, io.EOF
				}
			}
			errors <- event.UnknownEventTypeError{Type: "some-event"}
			errors <- event.UnknownEventVersionError{Type: "some-bad-version-event"}
		})

		It("prints the build's run script", func() {
			Expect(out.Contents()).To(ContainSubstring("failed to parse next event"))
		})

		It("exits with 255 exit code", func() {
			Expect(exitStatus).To(Equal(255))
		})

		Context("when IgnoreEventParsingErrors is configured", func() {
			BeforeEach(func() {
				options.IgnoreEventParsingErrors = true
			})
			It("exits with 0 exit code", func() {
				Expect(exitStatus).To(Equal(0))
			})

		})

	})

})
