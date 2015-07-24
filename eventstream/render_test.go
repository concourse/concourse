package eventstream_test

import (
	"io"
	"time"

	"github.com/concourse/atc"
	"github.com/concourse/atc/event"
	. "github.com/concourse/fly/eventstream"
	"github.com/concourse/fly/eventstream/fakes"
	"github.com/mgutz/ansi"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

var _ = Describe("V1.0 Renderer", func() {
	var (
		out    *gbytes.Buffer
		stream *fakes.FakeEventStream

		receivedEvents chan<- atc.Event

		exitStatus int
	)

	BeforeEach(func() {
		out = gbytes.NewBuffer()
		stream = new(fakes.FakeEventStream)

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
		exitStatus = Render(out, stream)
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

		It("prints its message in bold red, followed by a linebreak", func() {
			Ω(out.Contents()).Should(ContainSubstring(ansi.Color("oh no!", "red+b") + "\n"))
		})
	})

	Context("when an InitializeTask event is received", func() {
		BeforeEach(func() {
			receivedEvents <- event.InitializeTask{
				TaskConfig: event.TaskConfig{
					Image: "some-image",
					Run: event.TaskRunConfig{
						Path: "/some/script",
						Args: []string{"arg1", "arg2"},
					},
				},
			}
		})

		It("prints the build's container", func() {
			Ω(out.Contents()).Should(ContainSubstring("\x1b[1minitializing with some-image\x1b[0m\n"))
		})

		Context("and a StartExecute event is received", func() {
			BeforeEach(func() {
				receivedEvents <- event.StartTask{
					Time: time.Now().Unix(),
				}
			})

			It("prints the build's run script", func() {
				Ω(out.Contents()).Should(ContainSubstring("\x1b[1mrunning /some/script arg1 arg2\x1b[0m\n"))
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
			Ω(exitStatus).Should(Equal(42))
		})

		Context("and a Status event is received", func() {
			BeforeEach(func() {
				receivedEvents <- event.Status{
					Status: atc.StatusSucceeded,
				}
			})

			It("still processes it", func() {
				Ω(out.Contents()).Should(ContainSubstring("succeeded"))
			})

			It("exits with the status from the FinishTask event", func() {
				Ω(exitStatus).Should(Equal(42))
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
				Ω(out.Contents()).Should(ContainSubstring(ansi.Color("succeeded", "green") + "\n"))
			})

			It("exits 0", func() {
				Ω(exitStatus).Should(Equal(0))
			})
		})

		Context("with status 'failed'", func() {
			BeforeEach(func() {
				receivedEvents <- event.Status{
					Status: atc.StatusFailed,
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
					Status: atc.StatusErrored,
				}
			})

			It("prints it in magenta", func() {
				Ω(out.Contents()).Should(ContainSubstring(ansi.Color("errored", "magenta") + "\n"))
			})

			It("exits 2", func() {
				Ω(exitStatus).Should(Equal(2))
			})
		})

		Context("with status 'aborted'", func() {
			BeforeEach(func() {
				receivedEvents <- event.Status{
					Status: atc.StatusAborted,
				}
			})

			It("prints it in yellow", func() {
				Ω(out.Contents()).Should(ContainSubstring(ansi.Color("aborted", "yellow") + "\n"))
			})

			It("exits 3", func() {
				Ω(exitStatus).Should(Equal(3))
			})
		})
	})
})
