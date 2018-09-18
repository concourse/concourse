package eventstream_test

import (
	"io"
	"time"

	"github.com/fatih/color"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/onsi/gomega/gbytes"

	"github.com/concourse/atc"
	"github.com/concourse/atc/event"
	"github.com/concourse/fly/eventstream"
	"github.com/concourse/fly/ui"
	"github.com/concourse/go-concourse/concourse/eventstream/eventstreamfakes"
)

var _ = Describe("V1.0 Renderer", func() {
	var (
		out    *gbytes.Buffer
		stream *eventstreamfakes.FakeEventStream

		receivedEvents chan<- atc.Event

		exitStatus int
	)

	BeforeEach(func() {
		color.NoColor = false
		out = gbytes.NewBuffer()
		stream = new(eventstreamfakes.FakeEventStream)

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
		exitStatus = eventstream.Render(out, stream)
	})

	Context("when a Log event is received", func() {
		BeforeEach(func() {
			receivedEvents <- event.Log{
				Payload: "hello",
			}
		})

		It("prints its payload", func() {
			Expect(out).To(gbytes.Say("hello"))
		})
	})

	Context("when an Error event is received", func() {
		BeforeEach(func() {
			receivedEvents <- event.Error{
				Message: "oh no!",
			}
		})

		It("prints its message with a red background in white, followed by a linebreak", func() {
			Expect(out.Contents()).To(ContainSubstring(ui.ErroredColor.SprintFunc()("oh no!") + "\n"))
		})
	})

	Context("when an InitializeTask event is received", func() {
		BeforeEach(func() {
			receivedEvents <- event.InitializeTask{}
		})

		It("prints initializing", func() {
			Expect(out.Contents()).To(ContainSubstring("\x1b[1minitializing\x1b[0m\n"))
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
		})
	})

	Describe("receiving a Status event", func() {
		Context("with status 'succeeded'", func() {
			BeforeEach(func() {
				receivedEvents <- event.Status{
					Status: atc.StatusSucceeded,
				}
			})

			It("prints it in green", func() {
				Expect(out.Contents()).To(ContainSubstring(ui.SucceededColor.SprintFunc()("succeeded") + "\n"))
			})

			It("exits 0", func() {
				Expect(exitStatus).To(Equal(0))
			})
		})

		Context("with status 'failed'", func() {
			BeforeEach(func() {
				receivedEvents <- event.Status{
					Status: atc.StatusFailed,
				}
			})

			It("prints it in red", func() {
				Expect(out.Contents()).To(ContainSubstring(ui.FailedColor.SprintFunc()("failed") + "\n"))
			})

			It("exits 1", func() {
				Expect(exitStatus).To(Equal(1))
			})
		})

		Context("with status 'errored'", func() {
			BeforeEach(func() {
				receivedEvents <- event.Status{
					Status: atc.StatusErrored,
				}
			})

			It("prints it in magenta", func() {
				Expect(out.Contents()).To(ContainSubstring(ui.ErroredColor.SprintFunc()("errored") + "\n"))
			})

			It("exits 2", func() {
				Expect(exitStatus).To(Equal(2))
			})
		})

		Context("with status 'aborted'", func() {
			BeforeEach(func() {
				receivedEvents <- event.Status{
					Status: atc.StatusAborted,
				}
			})

			It("prints it in yellow", func() {
				Expect(out.Contents()).To(ContainSubstring(ui.AbortedColor.SprintFunc()("aborted") + "\n"))
			})

			It("exits 3", func() {
				Expect(exitStatus).To(Equal(3))
			})
		})
	})
})
