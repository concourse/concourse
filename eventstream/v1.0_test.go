package eventstream_test

import (
	"io"
	"time"

	. "github.com/concourse/fly/eventstream"
	"github.com/concourse/fly/eventstream/fakes"
	"github.com/concourse/turbine/api/builds"
	"github.com/concourse/turbine/event"
	"github.com/mgutz/ansi"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

var _ = Describe("V1.0 Renderer", func() {
	var (
		out    *gbytes.Buffer
		stream *fakes.FakeEventStream

		receivedEvents chan<- event.Event

		renderer Renderer

		exitStatus int
	)

	BeforeEach(func() {
		out = gbytes.NewBuffer()
		stream = new(fakes.FakeEventStream)

		events := make(chan event.Event, 100)
		receivedEvents = events

		stream.NextEventStub = func() (event.Event, error) {
			select {
			case ev := <-events:
				return ev, nil
			default:
				return nil, io.EOF
			}
		}

		renderer = V10Renderer{}
	})

	JustBeforeEach(func() {
		exitStatus = renderer.Render(out, stream)
	})

	Context("when a Log event is received", func() {
		BeforeEach(func() {
			receivedEvents <- event.Log{
				Payload: "hello",
			}
		})

		It("prints its payload", func() {
			Ω(out).Should(gbytes.Say("hello"))
		})
	})

	Context("when an Error event is received", func() {
		BeforeEach(func() {
			receivedEvents <- event.Error{
				Message: "oh no!",
			}
		})

		It("prints its message in bold red", func() {
			Ω(out.Contents()).Should(ContainSubstring(ansi.Color("oh no!", "red+b")))
		})
	})

	Context("when an Initialize event is received", func() {
		BeforeEach(func() {
			receivedEvents <- event.Initialize{
				BuildConfig: builds.Config{
					Image: "some-image",
					Run: builds.RunConfig{
						Path: "/some/script",
						Args: []string{"arg1", "arg2"},
					},
				},
			}
		})

		It("prints the build's container", func() {
			Ω(out.Contents()).Should(ContainSubstring("\x1b[1minitializing with some-image\x1b[0m\n"))
		})

		Context("and a Start event is received", func() {
			BeforeEach(func() {
				receivedEvents <- event.Start{
					Time: time.Now().Unix(),
				}
			})

			It("prints the build's container", func() {
				Ω(out.Contents()).Should(ContainSubstring("\x1b[1mrunning /some/script arg1 arg2\x1b[0m\n"))
			})
		})
	})

	Describe("receiving a Status event", func() {
		Context("with status 'succeeded'", func() {
			BeforeEach(func() {
				receivedEvents <- event.Status{
					Status: builds.StatusSucceeded,
				}
			})

			It("prints it in green", func() {
				Ω(out.Contents()).Should(ContainSubstring(ansi.Color("succeeded", "green") + "\n"))
			})

			It("exits 0", func() {
				Ω(exitStatus).Should(Equal(0))
			})
		})

		Context("with status 'failed'", func() {
			BeforeEach(func() {
				receivedEvents <- event.Status{
					Status: builds.StatusFailed,
				}
			})

			It("prints it in red", func() {
				Ω(out.Contents()).Should(ContainSubstring(ansi.Color("failed", "red") + "\n"))
			})

			It("exits 1", func() {
				Ω(exitStatus).Should(Equal(1))
			})
		})

		Context("with status 'errored'", func() {
			BeforeEach(func() {
				receivedEvents <- event.Status{
					Status: builds.StatusErrored,
				}
			})

			It("prints it in magenta", func() {
				Ω(out.Contents()).Should(ContainSubstring(ansi.Color("errored", "magenta") + "\n"))
			})

			It("exits 2", func() {
				Ω(exitStatus).Should(Equal(2))
			})
		})
	})
})
