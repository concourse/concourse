package logfanout_test

import (
	"errors"

	. "github.com/concourse/atc/logfanout"
	"github.com/concourse/atc/logfanout/fakes"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("RawSink", func() {
	var (
		conn *fakes.FakeJSONWriteCloser

		sink Sink
	)

	BeforeEach(func() {
		conn = new(fakes.FakeJSONWriteCloser)

		sink = NewRawSink(conn)
	})

	Describe("WriteMessage", func() {
		It("forwards raw JSON messages", func() {
			err := sink.WriteMessage(rawMSG(`not even correct, who cares`))
			Ω(err).ShouldNot(HaveOccurred())

			Ω(conn.WriteJSONCallCount()).Should(Equal(1))
			Ω(conn.WriteJSONArgsForCall(0)).Should(Equal(rawMSG(`not even correct, who cares`)))
		})
	})

	Describe("Close", func() {
		It("closes the backing connection", func() {
			Ω(conn.CloseCallCount()).Should(Equal(0))

			err := sink.Close()
			Ω(err).ShouldNot(HaveOccurred())

			Ω(conn.CloseCallCount()).Should(Equal(1))
		})

		Context("when the backing connection errors", func() {
			disaster := errors.New("oh no!")

			BeforeEach(func() {
				conn.CloseReturns(disaster)
			})

			It("returns the error", func() {
				Ω(sink.Close()).Should(Equal(disaster))
			})
		})
	})
})
