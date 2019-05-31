package eventstream_test

import (
	"io"
	"time"

	"github.com/fatih/color"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/onsi/gomega/gbytes"

	"github.com/concourse/concourse/v5/atc"
	"github.com/concourse/concourse/v5/atc/event"
	"github.com/concourse/concourse/v5/fly/eventstream"
	"github.com/concourse/concourse/v5/fly/ui"
	"github.com/concourse/concourse/v5/go-concourse/concourse/eventstream/eventstreamfakes"
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
})
