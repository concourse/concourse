package logfanout_test

import (
	"strings"

	. "github.com/concourse/atc/logfanout"

	"github.com/concourse/atc/logfanout/fakes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Logfanout", func() {
	var (
		logDB *fakes.FakeLogDB

		fanout *LogFanout
	)

	BeforeEach(func() {
		logDB = new(fakes.FakeLogDB)

		fanout = NewLogFanout(42, logDB)
	})

	Describe("WriteMessage", func() {
		It("appends the message to the build's log", func() {
			err := fanout.WriteMessage(rawMSG("wat"))
			Ω(err).ShouldNot(HaveOccurred())
		})
	})

	Context("when a sink is attached", func() {
		var sink *fakes.FakeSink

		BeforeEach(func() {
			sink = new(fakes.FakeSink)
		})

		JustBeforeEach(func() {
			err := fanout.Attach(sink)
			Ω(err).ShouldNot(HaveOccurred())
		})

		Describe("writing messages", func() {
			It("writes them out to anyone attached", func() {
				err := fanout.WriteMessage(rawMSG(`{"hello":1}`))
				Ω(err).ShouldNot(HaveOccurred())

				Ω(sink.WriteMessageCallCount()).Should(Equal(1))
				Ω(sink.WriteMessageArgsForCall(0)).Should(Equal(rawMSG(`{"hello":1}`)))
			})
		})

		Context("when there is a build log saved", func() {
			BeforeEach(func() {
				logDB.BuildLogReturns([]byte(`{"version":"1.0"}{"some":"saved log"}{"another":"message"}`), nil)
			})

			It("immediately sends its contents", func() {
				Ω(sink.WriteMessageCallCount()).Should(Equal(3))
				Ω(sink.WriteMessageArgsForCall(0)).Should(Equal(rawMSG(`{"version":"1.0"}`)))
				Ω(sink.WriteMessageArgsForCall(1)).Should(Equal(rawMSG(`{"some":"saved log"}`)))
				Ω(sink.WriteMessageArgsForCall(2)).Should(Equal(rawMSG(`{"another":"message"}`)))
			})

			Context("but it contains pre-event stream output (backwards compatibility", func() {
				longLog := strings.Repeat("x", 1025)

				BeforeEach(func() {
					logDB.BuildLogReturns([]byte(longLog), nil)
				})

				type versionMessage struct {
					Version string `json:"version"`
				}

				It("writes a 0.0 version followed by the contents, chunked", func() {
					Ω(sink.WriteMessageCallCount()).Should(Equal(3))
					Ω(sink.WriteMessageArgsForCall(0)).Should(Equal(rawMSG(`{"version":"0.0"}`)))
					Ω(sink.WriteMessageArgsForCall(1)).Should(Equal(rawMSG(`{"log":"` + longLog[0:1024] + `"}`)))
					Ω(sink.WriteMessageArgsForCall(2)).Should(Equal(rawMSG(`{"log":"x"}`)))
				})

				Context("when a unicode codepoint falls on the chunk boundary", func() {
					unicodeLongLog := longLog[0:1023] + "日本語"

					BeforeEach(func() {
						logDB.BuildLogReturns([]byte(unicodeLongLog), nil)
					})

					It("does not cut it in half", func() {
						Ω(sink.WriteMessageCallCount()).Should(Equal(2))
						Ω(sink.WriteMessageArgsForCall(0)).Should(Equal(rawMSG(`{"version":"0.0"}`)))
						Ω(sink.WriteMessageArgsForCall(1)).Should(Equal(rawMSG(`{"log":"` + unicodeLongLog + `"}`)))
					})
				})
			})

			Describe("closing", func() {
				BeforeEach(func() {
					err := fanout.Close()
					Ω(err).ShouldNot(HaveOccurred())
				})

				It("flushes the log and immediately closes", func() {
					Ω(sink.WriteMessageCallCount()).Should(Equal(3))
					Ω(sink.CloseCallCount()).Should(Equal(1))
				})
			})
		})

		Describe("closing", func() {
			It("closes attached sinks", func() {
				Ω(sink.CloseCallCount()).Should(Equal(0))

				err := fanout.Close()
				Ω(err).ShouldNot(HaveOccurred())

				Ω(sink.CloseCallCount()).Should(Equal(1))
			})
		})

		Context("and another is attached", func() {
			var secondSink *fakes.FakeSink

			BeforeEach(func() {
				secondSink = new(fakes.FakeSink)
			})

			JustBeforeEach(func() {
				err := fanout.Attach(secondSink)
				Ω(err).ShouldNot(HaveOccurred())
			})

			Describe("writing messages", func() {
				It("fans them out to anyone attached", func() {
					Ω(sink.WriteMessageCallCount()).Should(Equal(0))
					Ω(secondSink.WriteMessageCallCount()).Should(Equal(0))

					err := fanout.WriteMessage(rawMSG(`{"hello":1}`))
					Ω(err).ShouldNot(HaveOccurred())

					Ω(sink.WriteMessageCallCount()).Should(Equal(1))
					Ω(sink.WriteMessageArgsForCall(0)).Should(Equal(rawMSG(`{"hello":1}`)))

					Ω(secondSink.WriteMessageCallCount()).Should(Equal(1))
					Ω(secondSink.WriteMessageArgsForCall(0)).Should(Equal(rawMSG(`{"hello":1}`)))
				})
			})
		})
	})
})
